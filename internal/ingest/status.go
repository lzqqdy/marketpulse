package ingest

import "sync"

type statusTracker struct {
	mu sync.RWMutex
	m  map[string]string
}

func newStatusTracker() *statusTracker {
	return &statusTracker{m: make(map[string]string)}
}

func (t *statusTracker) set(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m[name] = value
}

func (t *statusTracker) snapshot() map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]string, len(t.m))
	for k, v := range t.m {
		out[k] = v
	}
	return out
}
