package store

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// MarketStore holds in-memory market data (thread-safe).
type MarketStore struct {
	mu        sync.RWMutex
	version   uint64
	quotes    map[string]Quote
	rates     Rates
	indices   []IndexQuote
	alpha     AlphaSnapshot
	macro     MacroSnapshot
	symbols   []string // display order
	listeners []ChangeListener
}

// UpdateAlphaQuote upserts a Binance Alpha quote without affecting crypto quotes.
func (s *MarketStore) UpdateAlphaQuote(row AlphaQuote) uint64 {
	row.Symbol = strings.ToUpper(strings.TrimSpace(row.Symbol))
	row.ID = strings.ToLower(strings.TrimSpace(row.ID))
	row.Category = strings.ToLower(strings.TrimSpace(row.Category))
	if row.Symbol == "" || row.ID == "" {
		return s.Version()
	}
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = time.Now().UTC()
	}
	if row.Source == "" {
		row.Source = "binance-alpha"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch row.Category {
	case "index":
		s.alpha.Indices = upsertAlphaQuote(s.alpha.Indices, row)
	case "stock":
		s.alpha.Stocks = upsertAlphaQuote(s.alpha.Stocks, row)
	default:
		return s.version
	}
	if row.UpdatedAt.After(s.alpha.UpdatedAt) || s.alpha.UpdatedAt.IsZero() {
		s.alpha.UpdatedAt = row.UpdatedAt
	}
	s.alpha.Source = "binance-alpha"
	v := s.bump()
	s.notifyLocked(v)
	return v
}

func upsertAlphaQuote(rows []AlphaQuote, row AlphaQuote) []AlphaQuote {
	for i := range rows {
		if rows[i].ID == row.ID || rows[i].Symbol == row.Symbol {
			rows[i] = row
			return rows
		}
	}
	return append(rows, row)
}

// SetAlphaDefaults seeds configured Alpha rows so the UI can render before ticks arrive.
func (s *MarketStore) SetAlphaDefaults(indices []AlphaQuote, stocks []AlphaQuote) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alpha.Indices = append([]AlphaQuote(nil), indices...)
	s.alpha.Stocks = append([]AlphaQuote(nil), stocks...)
	if len(indices) > 0 || len(stocks) > 0 {
		s.alpha.Source = "binance-alpha"
	}
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// New creates a store with optional symbol order for snapshot listing.
func New(symbols ...string) *MarketStore {
	order := make([]string, 0, len(symbols))
	seen := make(map[string]struct{}, len(symbols))
	for _, s := range symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		order = append(order, s)
	}
	return &MarketStore{
		quotes:  make(map[string]Quote),
		symbols: order,
	}
}

// Version returns the current monotonic snapshot version.
func (s *MarketStore) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *MarketStore) bump() uint64 {
	s.version++
	return s.version
}

// UpdateQuote upserts a spot quote and recomputes CNY when rates are set.
func (s *MarketStore) UpdateQuote(q Quote) uint64 {
	sym := strings.ToUpper(strings.TrimSpace(q.Symbol))
	if sym == "" {
		return s.Version()
	}
	q.Symbol = sym
	if q.UpdatedAt.IsZero() {
		q.UpdatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rates.USDTCNY > 0 {
		q.PriceCny = q.PriceUsdt * s.rates.USDTCNY
	}
	if old, ok := s.quotes[sym]; ok {
		if q.Rank == 0 {
			q.Rank = old.Rank
		}
		if q.IconURL == "" {
			q.IconURL = old.IconURL
		}
		if q.MarketCapUsd == 0 {
			q.MarketCapUsd = old.MarketCapUsd
		}
		if q.Volume24hUsd == 0 {
			q.Volume24hUsd = old.Volume24hUsd
		}
	}
	s.quotes[sym] = q
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// UpdateQuoteKeepDayPct updates live ticker fields but preserves changeDayPct
// while the Asia/Shanghai day-open cache is not ready.
func (s *MarketStore) UpdateQuoteKeepDayPct(q Quote) uint64 {
	sym := strings.ToUpper(strings.TrimSpace(q.Symbol))
	if sym == "" {
		return s.Version()
	}
	q.Symbol = sym
	if q.UpdatedAt.IsZero() {
		q.UpdatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.quotes[sym]; ok {
		q.ChangeDayPct = old.ChangeDayPct
		if q.Rank == 0 {
			q.Rank = old.Rank
		}
		if q.IconURL == "" {
			q.IconURL = old.IconURL
		}
		if q.MarketCapUsd == 0 {
			q.MarketCapUsd = old.MarketCapUsd
		}
		if q.Volume24hUsd == 0 {
			q.Volume24hUsd = old.Volume24hUsd
		}
	}
	if s.rates.USDTCNY > 0 {
		q.PriceCny = q.PriceUsdt * s.rates.USDTCNY
	}
	s.quotes[sym] = q
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// UpdateQuoteMetadata merges slow market metadata into existing quotes.
func (s *MarketStore) UpdateQuoteMetadata(rows []Quote) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, row := range rows {
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" {
			continue
		}
		q, exists := s.quotes[sym]
		q.Symbol = sym
		if row.Rank > 0 {
			q.Rank = row.Rank
		}
		if row.IconURL != "" {
			q.IconURL = row.IconURL
		}
		if row.MarketCapUsd > 0 {
			q.MarketCapUsd = row.MarketCapUsd
		}
		if row.Volume24hUsd > 0 {
			q.Volume24hUsd = row.Volume24hUsd
		}
		if !exists && !row.UpdatedAt.IsZero() {
			q.UpdatedAt = row.UpdatedAt
		}
		s.quotes[sym] = q
	}

	v := s.bump()
	s.notifyLocked(v)
	return v
}

// UpdateRates sets fiat rates and refreshes quote CNY fields.
func (s *MarketStore) UpdateRates(r Rates) uint64 {
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.rates = r
	for sym, q := range s.quotes {
		if r.USDTCNY > 0 {
			q.PriceCny = q.PriceUsdt * r.USDTCNY
		}
		s.quotes[sym] = q
	}
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// SetIndices replaces index rows.
func (s *MarketStore) SetIndices(indices []IndexQuote) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indices = append([]IndexQuote(nil), indices...)
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// SetMacro replaces macro snapshot.
func (s *MarketStore) SetMacro(m MacroSnapshot) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.LongShort.Ratio == 0 {
		m.LongShort = s.macro.LongShort
	}
	if m.TopLongShort.Ratio == 0 {
		m.TopLongShort = s.macro.TopLongShort
	}
	if m.Funding.Symbol == "" {
		m.Funding = s.macro.Funding
	}
	if m.OpenInterest.Symbol == "" {
		m.OpenInterest = s.macro.OpenInterest
	}
	if m.TakerBuySell.Symbol == "" {
		m.TakerBuySell = s.macro.TakerBuySell
	}
	if m.Liquidations.Window == "" {
		m.Liquidations = s.macro.Liquidations
	}
	s.macro = m
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// SetDerivatives updates futures indicators without replacing other macro fields.
func (s *MarketStore) SetDerivatives(f FundingRate, oi OpenInterest, taker TakerBuySell, ls LongShortRatio, top LongShortRatio) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Symbol != "" {
		s.macro.Funding = f
	}
	if oi.Symbol != "" {
		s.macro.OpenInterest = oi
	}
	if taker.Symbol != "" {
		s.macro.TakerBuySell = taker
	}
	if ls.Symbol != "" {
		s.macro.LongShort = ls
	}
	if top.Symbol != "" {
		s.macro.TopLongShort = top
	}
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// SetLiquidations updates rolling liquidation totals.
func (s *MarketStore) SetLiquidations(l Liquidations) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.macro.Liquidations = l
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// SetLongShort updates futures sentiment without replacing other macro fields.
func (s *MarketStore) SetLongShort(r LongShortRatio) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.macro.LongShort = r
	v := s.bump()
	s.notifyLocked(v)
	return v
}

// GetSnapshot returns a copy of the current market state.
func (s *MarketStore) GetSnapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quotes := make([]Quote, 0, len(s.quotes))
	if len(s.symbols) > 0 {
		for _, sym := range s.symbols {
			if q, ok := s.quotes[sym]; ok {
				quotes = append(quotes, q)
			}
		}
	} else {
		for _, q := range s.quotes {
			quotes = append(quotes, q)
		}
		sort.Slice(quotes, func(i, j int) bool {
			return quotes[i].Symbol < quotes[j].Symbol
		})
	}

	indices := append([]IndexQuote(nil), s.indices...)
	alpha := AlphaSnapshot{
		Indices:   append([]AlphaQuote(nil), s.alpha.Indices...),
		Stocks:    append([]AlphaQuote(nil), s.alpha.Stocks...),
		UpdatedAt: s.alpha.UpdatedAt,
		Source:    s.alpha.Source,
	}
	return Snapshot{
		Version: s.version,
		Ts:      time.Now().UnixMilli(),
		Quotes:  quotes,
		Rates:   s.rates,
		Indices: indices,
		Alpha:   alpha,
		Macro:   s.macro,
	}
}
