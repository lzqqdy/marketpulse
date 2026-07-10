package ingest

import (
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

type equityCache struct {
	mu   sync.RWMutex
	rows map[string]store.IndexQuote
}

func newEquityCache() *equityCache {
	return &equityCache{rows: make(map[string]store.IndexQuote)}
}

func (c *equityCache) fresh(defs []equity.IndexDef, now time.Time, ttlFor func(equity.IndexDef, time.Time) time.Duration) ([]store.IndexQuote, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]store.IndexQuote, 0, len(defs))
	for _, def := range defs {
		row := c.rows[def.ID]
		if !freshEnough(row, def, now, ttlFor) {
			return nil, false
		}
		row.Stale = false
		out = append(out, row)
	}
	return out, true
}

func (c *equityCache) expiredDefs(defs []equity.IndexDef, now time.Time, ttlFor func(equity.IndexDef, time.Time) time.Duration) []equity.IndexDef {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]equity.IndexDef, 0, len(defs))
	for _, def := range defs {
		row, ok := c.rows[def.ID]
		if !ok || !freshEnough(row, def, now, ttlFor) {
			out = append(out, def)
		}
	}
	return out
}

func freshEnough(row store.IndexQuote, def equity.IndexDef, now time.Time, ttlFor func(equity.IndexDef, time.Time) time.Duration) bool {
	if row.FetchedAt.IsZero() {
		return false
	}
	ttl := ttlFor(def, now)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return now.Sub(row.FetchedAt) <= ttl
}

func (c *equityCache) merge(rows map[string]store.IndexQuote, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, row := range rows {
		if row.ID == "" {
			row.ID = id
		}
		if row.FetchedAt.IsZero() {
			row.FetchedAt = now
		}
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = row.FetchedAt
		}
		if prev, ok := c.rows[row.ID]; ok && row.ChangePct == 0 && prev.ChangePct != 0 {
			row.ChangePct = prev.ChangePct
		}
		row.Stale = false
		c.rows[row.ID] = row
	}
}

func (c *equityCache) snapshot(defs []equity.IndexDef, stale bool) []store.IndexQuote {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]store.IndexQuote, 0, len(defs))
	for _, def := range defs {
		row, ok := c.rows[def.ID]
		if !ok {
			continue
		}
		row.Stale = stale || row.Stale
		out = append(out, row)
	}
	return out
}

type equityBreakers struct {
	mu     sync.Mutex
	states map[string]*providerBreaker
}

type providerBreaker struct {
	failCount int
	openUntil time.Time
}

func newEquityBreakers() *equityBreakers {
	return &equityBreakers{states: make(map[string]*providerBreaker)}
}

func (b *equityBreakers) isOpen(name string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.states[name]
	return st != nil && now.Before(st.openUntil)
}

func (b *equityBreakers) success(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.states[name] = &providerBreaker{}
}

func (b *equityBreakers) failure(name string, now time.Time, rateLimited bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.states[name]
	if st == nil {
		st = &providerBreaker{}
		b.states[name] = st
	}
	st.failCount++
	if rateLimited {
		st.openUntil = now.Add(20 * time.Minute)
		return
	}
	if st.failCount >= 3 {
		st.openUntil = now.Add(5 * time.Minute)
	}
}
