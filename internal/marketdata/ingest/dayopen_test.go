package ingest

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

func TestDayOpenCacheChangePctUsesShanghaiDate(t *testing.T) {
	c := newDayOpenCache()
	now := time.Date(2026, 5, 19, 0, 2, 0, 0, binance.Shanghai)
	c.replace("2026-05-19", map[string]float64{"BTC": 100}, "history_1m")

	pct, ok := c.changePct("BTC", 101, now)
	if !ok || pct < 0.99 || pct > 1.01 {
		t.Fatalf("pct=%v ok=%v", pct, ok)
	}

	previousDay := time.Date(2026, 5, 18, 23, 59, 0, 0, binance.Shanghai)
	if _, ok := c.changePct("BTC", 101, previousDay); ok {
		t.Fatal("previous Shanghai date should miss")
	}
}

func TestDayOpenCacheFallbackStillNeedsHistoricalRefresh(t *testing.T) {
	c := newDayOpenCache()
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, binance.Shanghai)
	c.setFallback("BTC", 100, now)

	pct, ok := c.changePct("BTC", 101, now)
	if !ok || pct < 0.99 || pct > 1.01 {
		t.Fatalf("fallback pct=%v ok=%v", pct, ok)
	}
	if !c.needsRefresh([]string{"BTC"}, now) {
		t.Fatal("fallback cache should still need historical refresh")
	}

	c.replace("2026-05-19", map[string]float64{"BTC": 99}, "history_1m")
	if c.needsRefresh([]string{"BTC"}, now) {
		t.Fatal("historical cache should not need refresh")
	}
	pct, ok = c.changePct("BTC", 101, now)
	if !ok || pct < 2.01 || pct > 2.03 {
		t.Fatalf("historical pct=%v ok=%v", pct, ok)
	}
}

func TestDayOpenCacheNeedsRefreshOnNewShanghaiDay(t *testing.T) {
	c := newDayOpenCache()
	c.replace("2026-05-18", map[string]float64{"BTC": 100}, "history_1m")

	now := time.Date(2026, 5, 19, 0, 1, 0, 0, binance.Shanghai)
	if !c.needsRefresh([]string{"BTC"}, now) {
		t.Fatal("new Shanghai day should need refresh")
	}
}
