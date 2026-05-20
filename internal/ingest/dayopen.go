package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/binance"
)

// dayOpenCache holds Asia/Shanghai natural-day open prices per symbol.
type dayOpenCache struct {
	mu      sync.RWMutex
	entries map[string]dayOpenEntry
}

type dayOpenEntry struct {
	date   string
	open   float64
	source string
}

func newDayOpenCache() *dayOpenCache {
	return &dayOpenCache{entries: make(map[string]dayOpenEntry)}
}

func (c *dayOpenCache) replace(date string, opens map[string]float64, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for symbol, open := range opens {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" || open <= 0 {
			continue
		}
		c.entries[dayOpenKey(sym, date)] = dayOpenEntry{date: date, open: open, source: source}
	}
}

func (c *dayOpenCache) setFallback(symbol string, price float64, now time.Time) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || price <= 0 {
		return
	}
	date := binance.ShanghaiDayKey(now)
	key := dayOpenKey(sym, date)

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry := c.entries[key]; entry.open > 0 {
		return
	}
	c.entries[key] = dayOpenEntry{date: date, open: price, source: "realtime_fallback"}
	slog.Info("shanghai day open fallback set",
		"symbol", sym,
		"date", date,
		"open", price,
		"timezone", "Asia/Shanghai",
	)
}

func (c *dayOpenCache) changePct(symbol string, price float64, now time.Time) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || price <= 0 {
		return 0, false
	}
	date := binance.ShanghaiDayKey(now)

	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[dayOpenKey(sym, date)]
	if !ok || entry.open <= 0 {
		return 0, false
	}
	return (price - entry.open) / entry.open * 100, true
}

// needsRefresh is true when any symbol lacks a historical Asia/Shanghai day open.
func (c *dayOpenCache) needsRefresh(symbols []string, now time.Time) bool {
	want := binance.ShanghaiDayKey(now)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, symbol := range symbols {
		sym := strings.ToUpper(strings.TrimSpace(symbol))
		if sym == "" {
			continue
		}
		entry := c.entries[dayOpenKey(sym, want)]
		if entry.open <= 0 || entry.source != "history_1m" {
			return true
		}
	}
	return false
}

func dayOpenKey(symbol, date string) string {
	return strings.ToUpper(strings.TrimSpace(symbol)) + ":" + date
}

func (s *Service) runDayOpenLoop(ctx context.Context) {
	const staleRetry = 30 * time.Second

	refresh := func() error {
		err := s.refreshDayOpens(ctx)
		if err != nil {
			slog.Warn("shanghai day open refresh failed", "err", err)
		}
		return err
	}

	_ = refresh()

	dayTimer := time.NewTimer(timeUntilNextShanghaiDay())
	defer dayTimer.Stop()

	staleTicker := time.NewTicker(staleRetry)
	defer staleTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dayTimer.C:
			_ = refresh()
			dayTimer.Reset(timeUntilNextShanghaiDay())
		case <-staleTicker.C:
			if s.dayOpen.needsRefresh(s.cfg.DayOpenSymbols(), time.Now()) {
				_ = refresh()
			}
		}
	}
}

func timeUntilNextShanghaiDay() time.Duration {
	wait := time.Until(binance.NextShanghaiDayStartUTC(time.Now()))
	if wait < time.Second {
		return time.Second
	}
	return wait
}

func (s *Service) refreshDayOpens(ctx context.Context) error {
	now := time.Now()
	dateKey := binance.ShanghaiDayKey(now)
	start := binance.ShanghaiDayStartUTC(now)

	symbols := s.cfg.DayOpenSymbols()
	opens := make(map[string]float64, len(symbols))
	var firstErr error
	for _, sym := range symbols {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		open, err := binance.FetchKlineOpenAt(sym, start)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s day open: %w", sym, err)
			}
			slog.Warn("shanghai day open symbol failed",
				"symbol", sym,
				"date", dateKey,
				"start_utc", start.Format(time.RFC3339),
				"err", err,
			)
			continue
		}
		opens[strings.ToUpper(sym)] = open
	}
	if len(opens) == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("day open: no symbols configured")
	}

	s.dayOpen.replace(dateKey, opens, "history_1m")
	s.recalculateShanghaiDayPct(now)
	slog.Info("shanghai day open loaded",
		"date", dateKey,
		"timezone", "Asia/Shanghai",
		"start_utc", start.Format(time.RFC3339),
		"symbols", len(opens),
		"sample_btc", opens["BTC"],
	)
	return firstErr
}

func (s *Service) recalculateShanghaiDayPct(now time.Time) {
	for _, q := range s.store.GetSnapshot().Quotes {
		if dayPct, ok := s.dayOpen.changePct(q.Symbol, q.PriceUsdt, now); ok {
			q.ChangeDayPct = dayPct
			s.store.UpdateQuote(q)
		}
	}
}
