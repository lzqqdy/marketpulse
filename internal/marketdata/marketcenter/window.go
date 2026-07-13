package marketcenter

import (
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
)

var marketIndexRef = map[string]string{
	MarketAB: "sh000001",
	MarketHK: "hsi",
	MarketUS: "dji",
}

// MarketActiveForMarket reports whether the tab's reference index is in session.
func MarketActiveForMarket(market string, now time.Time) bool {
	id, ok := marketIndexRef[market]
	if !ok {
		return false
	}
	return equity.IsMarketActive(id, now)
}

// CacheTTLForMarket mirrors equity index cache cadence for the given market tab.
func CacheTTLForMarket(market string, now time.Time) time.Duration {
	id, ok := marketIndexRef[market]
	if !ok {
		return equity.InactiveTTL
	}
	return equity.CacheTTL(equity.IndexDef{ID: id}, now)
}

// NextRefreshInterval aligns background refresh with equity ingest scheduling.
func NextRefreshInterval(now time.Time) time.Duration {
	defs := make([]equity.IndexDef, 0, len(refreshMarkets))
	for _, market := range refreshMarkets {
		if id, ok := marketIndexRef[market]; ok {
			defs = append(defs, equity.IndexDef{ID: id})
		}
	}
	if len(defs) == 0 {
		return equity.InactiveTTL
	}
	return equity.NextPollInterval(defs, now)
}
