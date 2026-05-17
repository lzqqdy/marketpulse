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

// dayOpenCache holds Beijing-day open prices per symbol.
type dayOpenCache struct {
	mu    sync.RWMutex
	date  string             // YYYY-MM-DD Asia/Shanghai
	opens map[string]float64 // base symbol -> USDT open at 00:00 CST
}

func newDayOpenCache() *dayOpenCache {
	return &dayOpenCache{opens: make(map[string]float64)}
}

func (c *dayOpenCache) replace(date string, opens map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.date = date
	c.opens = opens
}

func (c *dayOpenCache) changePct(symbol string, price float64, now time.Time) (float64, bool) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" || price <= 0 {
		return 0, false
	}
	want := binance.DayKeyShanghai(now)

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.date != want {
		return 0, false
	}
	open, ok := c.opens[sym]
	if !ok || open <= 0 {
		return 0, false
	}
	return (price - open) / open * 100, true
}

func (s *Service) runDayOpenLoop(ctx context.Context) {
	refresh := func() {
		if err := s.refreshDayOpens(ctx); err != nil {
			slog.Warn("beijing day open refresh failed", "err", err)
		}
	}
	refresh()

	for {
		wait := time.Until(binance.NextDayStartShanghai(time.Now()))
		if wait < time.Second {
			wait = time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			refresh()
		}
	}
}

func (s *Service) refreshDayOpens(ctx context.Context) error {
	now := time.Now()
	dayStart := binance.DayStartShanghai(now)
	dateKey := binance.DayKeyShanghai(now)

	opens := make(map[string]float64, len(s.cfg.Symbols))
	for _, sym := range s.cfg.Symbols {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		open, err := binance.FetchKlineOpenAt(sym, dayStart)
		if err != nil {
			return fmt.Errorf("%s day open: %w", sym, err)
		}
		opens[strings.ToUpper(sym)] = open
	}

	s.dayOpen.replace(dateKey, opens)
	slog.Info("beijing day open loaded",
		"date", dateKey,
		"symbols", len(opens),
		"sample_btc", opens["BTC"],
	)
	return nil
}
