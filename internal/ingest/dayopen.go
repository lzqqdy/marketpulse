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

// dayOpenCache holds Binance exchange-day open prices per symbol.
type dayOpenCache struct {
	mu    sync.RWMutex
	date  string             // YYYY-MM-DD UTC exchange day
	opens map[string]float64 // base symbol -> USDT open at 00:00 UTC
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
	want := binance.ExchangeDayKeyUTC(now)

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

// needsRefresh is true when the cache date does not match the current Binance exchange day.
func (c *dayOpenCache) needsRefresh(now time.Time) bool {
	want := binance.ExchangeDayKeyUTC(now)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.date != want || len(c.opens) == 0
}

func (s *Service) runDayOpenLoop(ctx context.Context) {
	const staleRetry = 30 * time.Second

	refresh := func() error {
		err := s.refreshDayOpens(ctx)
		if err != nil {
			slog.Warn("exchange day open refresh failed", "err", err)
		}
		return err
	}

	_ = refresh()

	dayTimer := time.NewTimer(timeUntilNextExchangeDay())
	defer dayTimer.Stop()

	staleTicker := time.NewTicker(staleRetry)
	defer staleTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dayTimer.C:
			_ = refresh()
			dayTimer.Reset(timeUntilNextExchangeDay())
		case <-staleTicker.C:
			if s.dayOpen.needsRefresh(time.Now()) {
				_ = refresh()
			}
		}
	}
}

func timeUntilNextExchangeDay() time.Duration {
	wait := time.Until(binance.NextExchangeDayStartUTC(time.Now()))
	if wait < time.Second {
		return time.Second
	}
	return wait
}

func (s *Service) refreshDayOpens(ctx context.Context) error {
	now := time.Now()
	dateKey := binance.ExchangeDayKeyUTC(now)

	opens := make(map[string]float64, len(s.cfg.Symbols))
	var firstErr error
	for _, sym := range s.cfg.Symbols {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		open, err := binance.FetchExchangeDayOpen(sym)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s day open: %w", sym, err)
			}
			slog.Warn("exchange day open symbol failed", "symbol", sym, "err", err)
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

	s.dayOpen.replace(dateKey, opens)
	slog.Info("exchange day open loaded",
		"date", dateKey,
		"symbols", len(opens),
		"sample_btc", opens["BTC"],
	)
	return firstErr
}
