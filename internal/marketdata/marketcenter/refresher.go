package marketcenter

import (
	"context"
	"log/slog"
	"time"
)

var refreshMarkets = []string{MarketAB, MarketHK, MarketUS}

// Start warms market center caches in the background.
func (c *Client) Start(ctx context.Context) {
	go c.runRefresher(ctx)
}

func (c *Client) runRefresher(ctx context.Context) {
	refresh := func() {
		now := time.Now()
		for _, market := range refreshMarkets {
			key := "center:" + market
			if _, ok := c.cache.getIfFresh(key, CacheTTLForMarket(market, now)); ok {
				continue
			}
			if _, err := c.Center(market); err != nil {
				slog.Warn("market center refresh failed", "market", market, "err", err)
			}
		}
	}
	refresh()
	for {
		wait := NextRefreshInterval(time.Now())
		if wait <= 0 {
			wait = time.Minute
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
