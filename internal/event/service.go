package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/config"
	eventmigrate "github.com/lzqqdy/marketpulse/internal/event/migrate"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

// Service is the public event module facade.
type Service interface {
	Enabled() bool
	Start(ctx context.Context)
	List(ctx context.Context, f ListFilter) (ListResult, error)
	Get(ctx context.Context, id string) (*MarketEvent, error)
	Timeline(ctx context.Context, id string) ([]TimelineItem, error)
	ServeWS(w http.ResponseWriter, r *http.Request)
}

type service struct {
	cfg    config.EventConfig
	md     marketdata.MarketDataService
	repo   *repository
	hub    *Hub
	liq    *LiquidationTracker
	engine *engine
}

// BootstrapArgs bundles deps for the event module.
type BootstrapArgs struct {
	Event      config.EventConfig
	DB         *sql.DB
	MarketData marketdata.MarketDataService
}

// Bootstrap opens migrations and builds the event service when enabled.
func Bootstrap(ctx context.Context, args BootstrapArgs) (Service, error) {
	cfg := args.Event
	if !cfg.Enabled {
		return &service{cfg: cfg}, nil
	}
	if args.DB == nil {
		return nil, fmt.Errorf("event: mysql required when event.enabled")
	}
	if args.MarketData == nil {
		return nil, fmt.Errorf("event: market data required when event.enabled")
	}
	if cfg.IsAutoMigrate() {
		if err := eventmigrate.Run(ctx, args.DB); err != nil {
			return nil, err
		}
		slog.Info("event migrations applied")
	}
	repo := newRepository(args.DB)
	hub := NewHub()
	liq := NewLiquidationTracker(cfg.Liquidation)
	svc := &service{
		cfg:  cfg,
		md:   args.MarketData,
		repo: repo,
		hub:  hub,
		liq:  liq,
	}
	svc.engine = newEngine(svc)
	return svc, nil
}

func (s *service) Enabled() bool {
	return s != nil && s.cfg.Enabled && s.repo != nil
}

func (s *service) Start(ctx context.Context) {
	if !s.Enabled() || s.engine == nil {
		return
	}
	s.engine.Start(ctx)
}

func (s *service) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if !s.Enabled() {
		return ListResult{}, errDisabled
	}
	return s.repo.List(ctx, f)
}

func (s *service) Get(ctx context.Context, id string) (*MarketEvent, error) {
	if !s.Enabled() {
		return nil, errDisabled
	}
	return s.repo.Get(ctx, id)
}

func (s *service) Timeline(ctx context.Context, id string) ([]TimelineItem, error) {
	ev, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	items := make([]TimelineItem, 0, len(ev.Signals)+2)
	items = append(items, TimelineItem{
		Ts:     ev.StartTime.Unix(),
		Kind:   "status",
		Label:  "事件开始",
		Status: StatusDetected,
		Payload: map[string]any{
			"score": ev.Score,
		},
	})
	for _, sig := range ev.Signals {
		items = append(items, TimelineItem{
			Ts:         sig.Timestamp.Unix(),
			Kind:       "signal",
			Label:      signalLabel(sig),
			SignalType: sig.SignalType,
			Symbol:     sig.Symbol,
			Payload: map[string]any{
				"changePct": sig.ChangePct,
				"ratio":     sig.Ratio,
				"window":    sig.Window,
			},
		})
	}
	if ev.Status == StatusResolved && ev.EndTime != nil {
		items = append(items, TimelineItem{
			Ts:     ev.EndTime.Unix(),
			Kind:   "status",
			Label:  "事件结束",
			Status: StatusResolved,
			Payload: map[string]any{
				"score": ev.Score,
			},
		})
	}
	return items, nil
}

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *service) ServeWS(w http.ResponseWriter, r *http.Request) {
	if !s.Enabled() {
		http.Error(w, "event disabled", http.StatusServiceUnavailable)
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.hub.Register(conn)
	defer func() {
		s.hub.Unregister(conn)
		_ = conn.Close()
	}()
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if strings.EqualFold(strings.TrimSpace(string(msg)), "ping") {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		}
	}
}

var (
	errDisabled = errors.New("event disabled")
)

func signalLabel(sig EventSignal) string {
	switch sig.SignalType {
	case SignalPriceDrop:
		return fmt.Sprintf("%s %s 价格下跌", sig.Symbol, sig.Window)
	case SignalPriceSpike:
		return fmt.Sprintf("%s %s 价格上涨", sig.Symbol, sig.Window)
	case SignalVolumeSpike:
		return fmt.Sprintf("%s 成交量放大", sig.Symbol)
	case SignalVolatilitySpike:
		return fmt.Sprintf("%s 波动放大", sig.Symbol)
	case SignalLiquidationSpike:
		return "全市场爆仓放大"
	default:
		return sig.SignalType
	}
}

func (s *service) broadcast(msgType string, ev MarketEvent) {
	payload := map[string]any{
		"type": msgType,
		"event": map[string]any{
			"id":        ev.ID,
			"type":      ev.Type,
			"subType":   ev.SubType,
			"title":     ev.Title,
			"severity":  ev.Severity,
			"score":     ev.Score,
			"status":    ev.Status,
			"symbols":   ev.Symbols,
			"markets":   ev.Markets,
			"startTime": ev.StartTime.Unix(),
			"updatedAt": ev.UpdatedAt.Unix(),
		},
	}
	if ev.EndTime != nil {
		payload["event"].(map[string]any)["endTime"] = ev.EndTime.Unix()
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.hub.Broadcast(b)
}

type engine struct {
	svc     *service
	mu      sync.Mutex
	running bool
}

func newEngine(svc *service) *engine {
	return &engine{svc: svc}
}

func (e *engine) Start(ctx context.Context) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.loop(ctx)
	slog.Info("event engine started", "interval", e.svc.cfg.EvaluateInterval, "symbols", e.svc.cfg.Symbols)
}

func (e *engine) loop(ctx context.Context) {
	ticker := time.NewTicker(e.svc.cfg.EvaluateInterval)
	defer ticker.Stop()
	// Initial delay so klines/macro have a chance to warm; also scrub legacy duplicates.
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			e.evaluate(ctx)
		case <-ticker.C:
			e.evaluate(ctx)
		}
	}
}

func (e *engine) evaluate(ctx context.Context) {
	svc := e.svc
	if !svc.Enabled() {
		return
	}
	now := time.Now().UTC()

	open, err := svc.repo.ListOpen(ctx)
	if err != nil {
		slog.Warn("event list open", "err", err)
		return
	}

	// Collapse duplicate OPEN rows per symbol (keep newest) + age/idle force-resolve.
	open = e.scrubOpen(ctx, open, now)

	signals := make([]EventSignal, 0, 16)

	intervals := make([]string, 0, len(svc.cfg.Price))
	for iv := range svc.cfg.Price {
		intervals = append(intervals, iv)
	}
	if len(intervals) == 0 {
		intervals = []string{"5m", "15m", "1h"}
	}

	for _, sym := range svc.cfg.Symbols {
		for _, iv := range intervals {
			cctx, cancel := context.WithTimeout(ctx, svc.cfg.KlineTimeout)
			_ = cctx
			klines, err := svc.md.Klines(sym, iv, 60)
			cancel()
			if err != nil || len(klines.Candles) == 0 {
				continue
			}
			priceCfg := config.EventPriceConfig{}
			if thr, ok := svc.cfg.Price[iv]; ok {
				priceCfg[iv] = thr
			}
			signals = append(signals, DetectPrice(sym, klines.Candles, priceCfg, now)...)
			if iv == "15m" || iv == "5m" {
				signals = append(signals, DetectVolume(sym, iv, klines.Candles, svc.cfg.Volume, now)...)
				signals = append(signals, DetectVolatility(sym, iv, klines.Candles, svc.cfg.Volatility, now)...)
			}
		}
	}

	snap := svc.md.Snapshot()
	signals = append(signals, svc.liq.Observe(snap.Macro.Liquidations.TotalUsd, now)...)

	// Ensure template IDs for open events.
	for i := range open {
		if open[i].TemplateID == "" {
			open[i].TemplateID = inferTemplateID(&open[i])
		}
	}

	agg := Aggregate(AggregateInput{
		Now:             now,
		Signals:         signals,
		OpenEvents:      open,
		AggregateWindow: svc.cfg.AggregateWindow,
	})

	openBySym := map[string]string{}
	for _, ev := range open {
		sym := primarySymbol(&ev)
		openBySym[sym] = ev.ID
	}

	for _, ev := range agg.Created {
		evCopy := ev
		sym := primarySymbol(&evCopy)
		// DB-level guard: never create a second OPEN for the same symbol.
		if id, ok := openBySym[sym]; ok {
			evCopy.ID = id
			if existing := findOpen(open, id); existing != nil {
				evCopy.StartTime = existing.StartTime
				evCopy.CreatedAt = existing.CreatedAt
				evCopy.PeakScore = existing.PeakScore
				if evCopy.Score > evCopy.PeakScore {
					evCopy.PeakScore = evCopy.Score
				}
				evCopy.Signals = append(append([]EventSignal{}, existing.Signals...), evCopy.Signals...)
			}
			if err := svc.repo.UpdateEvent(ctx, &evCopy); err != nil {
				slog.Warn("event create→merge update", "err", err, "id", evCopy.ID)
				continue
			}
			if sigs := agg.NewSignals[ev.ID]; len(sigs) > 0 {
				for i := range sigs {
					sigs[i].EventID = evCopy.ID
				}
				if err := svc.repo.InsertSignals(ctx, sigs); err != nil {
					slog.Warn("event insert signals", "err", err, "id", evCopy.ID)
				}
			}
			svc.broadcast("event.updated", evCopy)
			continue
		}
		if err := svc.repo.CreateEvent(ctx, &evCopy); err != nil {
			slog.Warn("event create", "err", err, "id", evCopy.ID)
			continue
		}
		openBySym[sym] = evCopy.ID
		if sigs := agg.NewSignals[evCopy.ID]; len(sigs) > 0 {
			if err := svc.repo.InsertSignals(ctx, sigs); err != nil {
				slog.Warn("event insert signals", "err", err, "id", evCopy.ID)
			}
		}
		svc.broadcast("event.created", evCopy)
	}
	for _, ev := range agg.Updated {
		evCopy := ev
		if err := svc.repo.UpdateEvent(ctx, &evCopy); err != nil {
			slog.Warn("event update", "err", err, "id", evCopy.ID)
			continue
		}
		if sigs := agg.NewSignals[evCopy.ID]; len(sigs) > 0 {
			if err := svc.repo.InsertSignals(ctx, sigs); err != nil {
				slog.Warn("event insert signals", "err", err, "id", evCopy.ID)
			}
		}
		msg := "event.updated"
		if evCopy.Status == StatusResolved {
			msg = "event.resolved"
		}
		svc.broadcast(msg, evCopy)
	}

	// Re-score open events that got no updates this round but may have cooled off.
	updatedIDs := map[string]struct{}{}
	for _, ev := range agg.Updated {
		updatedIDs[ev.ID] = struct{}{}
	}
	for _, ev := range open {
		if _, ok := updatedIDs[ev.ID]; ok {
			continue
		}
		if ev.Status == StatusResolved {
			continue
		}
		// Without fresh confirmation, decay harder so DEESCALATING can finish.
		score, _ := ScoreFromSignals(ev.Signals)
		cooled := score * 0.45
		before := ev.Status
		ApplyLifecycle(&ev, cooled, now)
		if ShouldForceResolve(&ev, now, svc.cfg.MaxOpenAge, svc.cfg.MaxDeescalateAge) {
			ForceResolve(&ev, now)
		}
		if ev.Status == before && ev.Score == cooled {
			continue
		}
		if err := svc.repo.UpdateEvent(ctx, &ev); err != nil {
			continue
		}
		msg := "event.updated"
		if ev.Status == StatusResolved {
			msg = "event.resolved"
		}
		svc.broadcast(msg, ev)
	}
}

func (e *engine) scrubOpen(ctx context.Context, open []MarketEvent, now time.Time) []MarketEvent {
	svc := e.svc
	keep := map[string]*MarketEvent{} // symbol → newest open
	dupIDs := make([]string, 0)
	staleIDs := make([]string, 0)

	for i := range open {
		ev := &open[i]
		if ShouldForceResolve(ev, now, svc.cfg.MaxOpenAge, svc.cfg.MaxDeescalateAge) {
			staleIDs = append(staleIDs, ev.ID)
			continue
		}
		sym := primarySymbol(ev)
		prev, ok := keep[sym]
		if !ok {
			keep[sym] = ev
			continue
		}
		// Both open: keep the newer start_time, resolve the other.
		if ev.StartTime.After(prev.StartTime) {
			dupIDs = append(dupIDs, prev.ID)
			keep[sym] = ev
		} else {
			dupIDs = append(dupIDs, ev.ID)
		}
	}

	resolveIDs := append(dupIDs, staleIDs...)
	if len(resolveIDs) > 0 {
		if err := svc.repo.ResolveEventIDs(ctx, resolveIDs, now); err != nil {
			slog.Warn("event scrub resolve", "err", err, "n", len(resolveIDs))
		} else {
			slog.Info("event scrubbed open rows", "duplicates", len(dupIDs), "stale", len(staleIDs))
			for _, id := range resolveIDs {
				if ev := findOpen(open, id); ev != nil {
					ForceResolve(ev, now)
					svc.broadcast("event.resolved", *ev)
				}
			}
		}
	}

	out := make([]MarketEvent, 0, len(keep))
	for _, ev := range keep {
		if ev.Status == StatusResolved {
			continue
		}
		// skip ids we just resolved
		skip := false
		for _, id := range resolveIDs {
			if ev.ID == id {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, *ev)
	}
	return out
}

func findOpen(open []MarketEvent, id string) *MarketEvent {
	for i := range open {
		if open[i].ID == id {
			return &open[i]
		}
	}
	return nil
}
