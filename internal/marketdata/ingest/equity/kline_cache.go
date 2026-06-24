package equity

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

const eastmoneyKlineRefreshLimit = 10

var defaultKlineCache = newKlineCache()

type klineCache struct {
	mu      sync.Mutex
	entries map[string]klineCacheEntry
}

type klineCacheEntry struct {
	candles   []binance.Candle
	fetchedAt time.Time
}

func newKlineCache() *klineCache {
	return &klineCache{entries: make(map[string]klineCacheEntry)}
}

// FetchCachedEastmoneyKlines caches immutable historical bars and refreshes only
// the latest few bars after the market-specific TTL expires.
func FetchCachedEastmoneyKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, string, error) {
	return defaultKlineCache.fetch(client, def, interval, limit, time.Now())
}

func (c *klineCache) fetch(client *http.Client, def IndexDef, interval string, limit int, now time.Time) ([]binance.Candle, string, error) {
	interval = strings.ToLower(strings.TrimSpace(interval))
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1000 {
		limit = 1000
	}
	key := fmt.Sprintf("%s:%s", strings.ToLower(def.ID), interval)
	ttl := CacheTTL(def, now)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if ok && now.Sub(entry.fetchedAt) <= ttl && len(entry.candles) >= limit {
		return trimCandles(entry.candles, limit), "eastmoney_cache", nil
	}

	fetchLimit := limit
	if ok && len(entry.candles) >= limit {
		fetchLimit = eastmoneyKlineRefreshLimit
		if fetchLimit > limit {
			fetchLimit = limit
		}
	}

	fresh, source, err := fetchIndexKlines(client, def, interval, fetchLimit)
	if err != nil {
		if ok && len(entry.candles) > 0 {
			return trimCandles(entry.candles, limit), "eastmoney_cache_stale", nil
		}
		return nil, "", err
	}

	merged := fresh
	if ok && len(entry.candles) > 0 && fetchLimit < limit {
		merged = mergeCandles(entry.candles, fresh)
	}
	merged = trimCandles(merged, limit)
	c.entries[key] = klineCacheEntry{
		candles:   append([]binance.Candle(nil), merged...),
		fetchedAt: now,
	}
	return append([]binance.Candle(nil), merged...), source, nil
}

func fetchIndexKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, string, error) {
	candles, err := FetchEastmoneyKlines(client, def, interval, limit)
	if err == nil {
		return candles, "eastmoney", nil
	}
	if interval != "1d" && interval != "" {
		return nil, "", err
	}
	tcandles, terr := FetchTencentKlines(client, def, interval, limit)
	if terr != nil {
		return nil, "", err
	}
	return tcandles, "tencent", nil
}

func mergeCandles(oldRows, newRows []binance.Candle) []binance.Candle {
	if len(oldRows) == 0 {
		return append([]binance.Candle(nil), newRows...)
	}
	if len(newRows) == 0 {
		return append([]binance.Candle(nil), oldRows...)
	}
	byTime := make(map[int64]binance.Candle, len(oldRows)+len(newRows))
	order := make([]int64, 0, len(oldRows)+len(newRows))
	for _, row := range oldRows {
		if _, ok := byTime[row.Time]; !ok {
			order = append(order, row.Time)
		}
		byTime[row.Time] = row
	}
	for _, row := range newRows {
		if _, ok := byTime[row.Time]; !ok {
			order = append(order, row.Time)
		}
		byTime[row.Time] = row
	}
	sortInt64s(order)
	out := make([]binance.Candle, 0, len(order))
	for _, ts := range order {
		out = append(out, byTime[ts])
	}
	return out
}

func trimCandles(rows []binance.Candle, limit int) []binance.Candle {
	if limit <= 0 || len(rows) <= limit {
		return append([]binance.Candle(nil), rows...)
	}
	return append([]binance.Candle(nil), rows[len(rows)-limit:]...)
}

func sortInt64s(values []int64) {
	for i := 1; i < len(values); i++ {
		v := values[i]
		j := i - 1
		for j >= 0 && values[j] > v {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = v
	}
}
