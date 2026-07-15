package alerts

import (
	"sync"
	"time"
)

const windowDuration = 5 * time.Minute

type pricePoint struct {
	ts    time.Time
	price float64
}

// Window5m tracks rolling high/low over five minutes for type-5 rules.
type Window5m struct {
	points []pricePoint
}

// Update records a price and returns current amplitude % and whether the window has enough data.
func (w *Window5m) Update(price float64, now time.Time) (ampPct float64, ready bool) {
	if w == nil {
		w = &Window5m{}
	}
	w.points = append(w.points, pricePoint{ts: now, price: price})
	cutoff := now.Add(-windowDuration)
	kept := w.points[:0]
	for _, p := range w.points {
		if !p.ts.Before(cutoff) {
			kept = append(kept, p)
		}
	}
	w.points = kept
	if len(w.points) < 2 {
		return 0, false
	}
	low, high := w.points[0].price, w.points[0].price
	for _, p := range w.points[1:] {
		if p.price < low {
			low = p.price
		}
		if p.price > high {
			high = p.price
		}
	}
	if low <= 0 {
		return 0, false
	}
	return (high - low) / low * 100, true
}

// WindowTracker holds per-key rolling windows.
type WindowTracker struct {
	mu sync.Mutex
	m  map[string]*Window5m
}

func NewWindowTracker() *WindowTracker {
	return &WindowTracker{m: make(map[string]*Window5m)}
}

func (t *WindowTracker) Update(key string, price float64, now time.Time) (ampPct float64, ready bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	w, ok := t.m[key]
	if !ok {
		w = &Window5m{}
		t.m[key] = w
	}
	return w.Update(price, now)
}

func (t *WindowTracker) Snapshot(key string) (ampPct float64, ready bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	w, ok := t.m[key]
	if !ok || len(w.points) < 2 {
		return 0, false
	}
	now := time.Now()
	return w.Update(w.points[len(w.points)-1].price, now)
}
