package marketcenter

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
)

func cst(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, time.FixedZone("CST", 8*3600))
}

func TestCacheTTLForMarket(t *testing.T) {
	open := cst(2026, time.July, 10, 10, 0)
	closed := cst(2026, time.July, 10, 18, 0)
	if got := CacheTTLForMarket(MarketAB, open); got != equity.ActiveTTL {
		t.Fatalf("ab open got %s", got)
	}
	if got := CacheTTLForMarket(MarketAB, closed); got != equity.InactiveTTL {
		t.Fatalf("ab closed got %s", got)
	}
	if got := CacheTTLForMarket(MarketUS, cst(2026, time.July, 10, 22, 0)); got != equity.ActiveTTL {
		t.Fatalf("us evening got %s", got)
	}
}
