package ingest

import (
	"sort"
	"sync"
	"time"
)

const (
	ProviderHealthy     = "healthy"
	ProviderStale       = "stale"
	ProviderCircuitOpen = "circuit_open"
	ProviderUnavailable = "unavailable"
	ProviderDisabled    = "disabled"
)

type ProviderHealthStore struct {
	mu        sync.RWMutex
	providers map[string]*ProviderHealth
}

type ProviderHealth struct {
	Name          string        `json:"name"`
	Label         string        `json:"label"`
	Category      string        `json:"category"`
	Status        string        `json:"status"`
	Role          string        `json:"role"`
	CurrentUsed   bool          `json:"current_used"`
	LatencyMs     int64         `json:"latency_ms"`
	LastSuccessAt int64         `json:"last_success_at"`
	LastErrorAt   int64         `json:"last_error_at"`
	LastError     string        `json:"last_error"`
	FailCount     int           `json:"fail_count"`
	CircuitOpen   bool          `json:"circuit_open"`
	StaleSeconds  int64         `json:"stale_seconds"`
	staleAfter    time.Duration `json:"-"`
	disabled      bool          `json:"-"`
}

type ProviderStatusResponse struct {
	Overall   ProviderOverall  `json:"overall"`
	Providers []ProviderHealth `json:"providers"`
}

type ProviderOverall struct {
	Status       string `json:"status"`
	Healthy      int    `json:"healthy"`
	Total        int    `json:"total"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
	UpdatedAt    int64  `json:"updated_at"`
}

type providerDef struct {
	Name       string
	Label      string
	Category   string
	Role       string
	StaleAfter time.Duration
	Disabled   bool
}

func newProviderHealthStore(defs []providerDef) *ProviderHealthStore {
	h := &ProviderHealthStore{providers: make(map[string]*ProviderHealth, len(defs))}
	for _, def := range defs {
		h.providers[def.Name] = &ProviderHealth{
			Name:       def.Name,
			Label:      def.Label,
			Category:   def.Category,
			Role:       def.Role,
			Status:     ProviderUnavailable,
			staleAfter: def.StaleAfter,
			disabled:   def.Disabled,
		}
		if def.Disabled {
			h.providers[def.Name].Status = ProviderDisabled
		}
	}
	return h
}

func defaultProviderDefs(alphaEnabled bool) []providerDef {
	return []providerDef{
		{Name: "binance_spot_ws", Label: "Binance Spot WS", Category: "crypto", Role: "primary", StaleAfter: 45 * time.Second},
		{Name: "okx_c2c", Label: "OKX C2C", Category: "forex", Role: "primary", StaleAfter: 3 * time.Minute},
		{Name: "frankfurter_fx", Label: "Frankfurter FX", Category: "forex", Role: "primary", StaleAfter: 15 * time.Minute},
		{Name: "tencent_index", Label: "Tencent", Category: "index", Role: "primary", StaleAfter: 3 * time.Minute},
		{Name: "sina_index", Label: "Sina", Category: "index", Role: "fallback", StaleAfter: 3 * time.Minute},
		{Name: "eastmoney_index", Label: "Eastmoney", Category: "index", Role: "fallback", StaleAfter: 3 * time.Minute},
		{Name: "sge_gold", Label: "SGE Gold", Category: "index", Role: "auxiliary", StaleAfter: 90 * time.Minute},
		{Name: "coingecko_macro", Label: "CoinGecko Macro", Category: "macro", Role: "primary", StaleAfter: 10 * time.Minute},
		{Name: "coingecko_meta", Label: "CoinGecko Metadata", Category: "macro", Role: "auxiliary", StaleAfter: 20 * time.Minute},
		{Name: "binance_alpha", Label: "Binance Alpha", Category: "alpha", Role: "primary", StaleAfter: 90 * time.Second, Disabled: !alphaEnabled},
		{Name: "binance_derivatives", Label: "Binance Derivatives", Category: "derivatives", Role: "primary", StaleAfter: 3 * time.Minute},
		{Name: "binance_liquidations", Label: "Binance Liquidations", Category: "derivatives", Role: "auxiliary", StaleAfter: 3 * time.Minute},
	}
}

func (h *ProviderHealthStore) ReportSuccess(name string, latency time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.ensure(name)
	row.disabled = false
	row.Status = ProviderHealthy
	row.LatencyMs = max(0, latency.Milliseconds())
	row.LastSuccessAt = time.Now().Unix()
	row.LastError = ""
	row.FailCount = 0
	row.CircuitOpen = false
}

func (h *ProviderHealthStore) ReportFailure(name string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.ensure(name)
	row.disabled = false
	row.LastErrorAt = time.Now().Unix()
	if err != nil {
		row.LastError = err.Error()
	} else {
		row.LastError = "unknown error"
	}
	row.FailCount++
	if row.LastSuccessAt == 0 {
		row.Status = ProviderUnavailable
	}
}

func (h *ProviderHealthStore) ReportCircuitOpen(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.ensure(name)
	row.CircuitOpen = true
	row.Status = ProviderCircuitOpen
}

func (h *ProviderHealthStore) ReportDisabled(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.ensure(name)
	row.disabled = true
	row.Status = ProviderDisabled
	row.LastError = ""
}

func (h *ProviderHealthStore) ReportUsed(name string, used bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ensure(name).CurrentUsed = used
}

func (h *ProviderHealthStore) Snapshot(now time.Time) ProviderStatusResponse {
	h.mu.RLock()
	rows := make([]ProviderHealth, 0, len(h.providers))
	for _, p := range h.providers {
		row := *p
		row.Status = computedProviderStatus(row, now)
		if row.LastSuccessAt > 0 {
			row.StaleSeconds = int64(now.Sub(time.Unix(row.LastSuccessAt, 0)).Seconds())
		}
		rows = append(rows, row)
	}
	h.mu.RUnlock()

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Category != rows[j].Category {
			return rows[i].Category < rows[j].Category
		}
		return rows[i].Name < rows[j].Name
	})

	var healthy int
	var latencySum int64
	var latencyCount int64
	for _, row := range rows {
		if row.Status == ProviderHealthy {
			healthy++
		}
		if row.LatencyMs > 0 && row.Status != ProviderDisabled {
			latencySum += row.LatencyMs
			latencyCount++
		}
	}
	avg := int64(0)
	if latencyCount > 0 {
		avg = latencySum / latencyCount
	}
	return ProviderStatusResponse{
		Overall: ProviderOverall{
			Status:       overallProviderStatus(healthy, len(rows)),
			Healthy:      healthy,
			Total:        len(rows),
			AvgLatencyMs: avg,
			UpdatedAt:    now.Unix(),
		},
		Providers: rows,
	}
}

func (h *ProviderHealthStore) ensure(name string) *ProviderHealth {
	if row, ok := h.providers[name]; ok {
		return row
	}
	row := &ProviderHealth{
		Name:       name,
		Label:      name,
		Category:   "other",
		Role:       "auxiliary",
		Status:     ProviderUnavailable,
		staleAfter: 5 * time.Minute,
	}
	h.providers[name] = row
	return row
}

func computedProviderStatus(row ProviderHealth, now time.Time) string {
	if row.disabled {
		return ProviderDisabled
	}
	if row.CircuitOpen {
		return ProviderCircuitOpen
	}
	if row.LastSuccessAt == 0 {
		return ProviderUnavailable
	}
	staleAfter := row.staleAfter
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}
	if now.Sub(time.Unix(row.LastSuccessAt, 0)) > staleAfter {
		return ProviderStale
	}
	return ProviderHealthy
}

func overallProviderStatus(healthy, total int) string {
	if total == 0 || healthy == 0 {
		return ProviderUnavailable
	}
	if healthy == total {
		return ProviderHealthy
	}
	return "degraded"
}
