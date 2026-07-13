package marketcenter

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
)

func TestCacheStaleAfterActiveTTL(t *testing.T) {
	cache := newResponseCache()
	key := "center:" + MarketAB
	cache.mu.Lock()
	cache.data[key] = cacheEntry{
		value:    CenterResponse{Market: MarketAB},
		cachedAt: time.Now().Add(-30 * time.Minute),
	}
	cache.mu.Unlock()

	if _, ok := cache.getIfFresh(key, equity.ActiveTTL); ok {
		t.Fatal("30 minute old entry should not satisfy 1 minute active ttl")
	}
}

func TestCacheFreshWithinInactiveTTL(t *testing.T) {
	cache := newResponseCache()
	key := "center:" + MarketAB
	cache.mu.Lock()
	cache.data[key] = cacheEntry{
		value:    CenterResponse{Market: MarketAB},
		cachedAt: time.Now().Add(-30 * time.Minute),
	}
	cache.mu.Unlock()

	if _, ok := cache.getIfFresh(key, equity.InactiveTTL); !ok {
		t.Fatal("30 minute old entry should remain fresh under 1 hour inactive ttl")
	}
}

func TestMarketActiveForMarket(t *testing.T) {
	// Monday 21:43 CST — A-share closed, US active.
	at2143 := cst(2026, time.July, 13, 21, 43)
	if MarketActiveForMarket(MarketAB, at2143) {
		t.Fatal("A-share should be closed at 21:43")
	}
	if !MarketActiveForMarket(MarketUS, at2143) {
		t.Fatal("US should be active at 21:43")
	}
	at911 := cst(2026, time.July, 13, 9, 11)
	if MarketActiveForMarket(MarketAB, at911) {
		t.Fatal("A-share should be closed before 9:15")
	}
}
