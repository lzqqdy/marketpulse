package marketcenter

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value    any
	cachedAt time.Time
}

type responseCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

func newResponseCache() *responseCache {
	return &responseCache{data: make(map[string]cacheEntry)}
}

func (c *responseCache) get(key string) (any, bool) {
	return c.getIfFresh(key, time.Hour)
}

func (c *responseCache) getIfFresh(key string, maxAge time.Duration) (any, bool) {
	if maxAge <= 0 {
		maxAge = time.Minute
	}
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.cachedAt) > maxAge {
		return nil, false
	}
	return entry.value, true
}

func (c *responseCache) set(key string, value any) {
	c.mu.Lock()
	c.data[key] = cacheEntry{value: value, cachedAt: time.Now()}
	c.mu.Unlock()
}
