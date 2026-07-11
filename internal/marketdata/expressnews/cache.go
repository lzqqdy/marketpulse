package expressnews

import (
	"sync"
	"time"
)

const (
	ttlFreshPage0  = 30 * time.Second
	ttlStablePage0 = 2 * time.Minute
	ttlHistoryPage = 10 * time.Minute
)

type cacheEntry struct {
	value     Response
	expiresAt time.Time
}

type responseCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

func newResponseCache() *responseCache {
	return &responseCache{data: make(map[string]cacheEntry)}
}

func (c *responseCache) get(key string) (Response, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return Response{}, false
	}
	return entry.value, true
}

func (c *responseCache) set(key string, value Response, ttl time.Duration) {
	if ttl <= 0 {
		ttl = ttlFreshPage0
	}
	c.mu.Lock()
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

type fingerprintStore struct {
	mu   sync.Mutex
	byTag map[string]string
}

func newFingerprintStore() *fingerprintStore {
	return &fingerprintStore{byTag: make(map[string]string)}
}

func (f *fingerprintStore) get(tag string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byTag[tag]
}

func (f *fingerprintStore) set(tag string, fp string) {
	f.mu.Lock()
	f.byTag[tag] = fp
	f.mu.Unlock()
}

func ttlForPage(tag string, pn int, latestID string, fp *fingerprintStore) time.Duration {
	if pn > 0 {
		return ttlHistoryPage
	}
	prev := fp.get(tag)
	if latestID != "" && prev == latestID {
		return ttlStablePage0
	}
	if latestID != "" {
		fp.set(tag, latestID)
	}
	return ttlFreshPage0
}
