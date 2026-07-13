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

func TestCacheTTLForMarketAt940Monday(t *testing.T) {
	at940 := cst(2026, time.July, 13, 9, 40) // Monday
	if got := CacheTTLForMarket(MarketAB, at940); got != equity.ActiveTTL {
		t.Fatalf("A-share tab at 9:40 should use active ttl, got %s", got)
	}
	at911 := cst(2026, time.July, 13, 9, 11)
	if got := CacheTTLForMarket(MarketAB, at911); got != equity.InactiveTTL {
		t.Fatalf("A-share tab at 9:11 should use inactive ttl, got %s", got)
	}
}
