package marketcenter

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type responseCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

func newResponseCache() *responseCache {
	return &responseCache{data: make(map[string]cacheEntry)}
}

func (c *responseCache) get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (c *responseCache) set(key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	c.mu.Lock()
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}
