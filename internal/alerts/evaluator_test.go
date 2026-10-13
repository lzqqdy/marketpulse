package alerts

import (
	"context"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
	"sync/atomic"
	"testing"
	"time"
)

type evaluationMarket struct {
	marketdata.MarketDataService
	snap marketdata.Snapshot
}

func (m *evaluationMarket) Snapshot() marketdata.Snapshot    { return m.snap }
func (m *evaluationMarket) AddListener(store.ChangeListener) {}
func (m *evaluationMarket) Quote(string) (marketdata.Quote, bool) {
	if len(m.snap.Quotes) > 0 {
		return m.snap.Quotes[0], true
	}
	return marketdata.Quote{}, false
}
func (m *evaluationMarket) IndexQuote(string) (marketdata.IndexQuote, bool) {
	if len(m.snap.Indices) > 0 {
		return m.snap.Indices[0], true
	}
	return marketdata.IndexQuote{}, false
}
func (m *evaluationMarket) AlphaQuote(string) (marketdata.AlphaQuote, bool) {
	if len(m.snap.Alpha.Stocks) > 0 {
		return m.snap.Alpha.Stocks[0], true
	}
	return marketdata.AlphaQuote{}, false
}

type evaluationQueue struct{ rules []Rule }

func (q *evaluationQueue) Enqueue(r Rule, _ float64, _ triggerMeta) bool {
	q.rules = append(q.rules, r)
	return true
}

type evaluationCooldown struct{ active bool }

func (c *evaluationCooldown) TrySet(context.Context, Rule) (bool, error) {
	if c.active {
		return false, nil
	}
	c.active = true
	return true, nil
}
func (c *evaluationCooldown) Clear(context.Context, int64) error { c.active = false; return nil }

func TestUnchangedPriceEvaluatesAfterCooldownAndReenable(t *testing.T) {
	for _, asset := range []string{AssetSpot, AssetIndex, AssetAlpha} {
		t.Run(asset, func(t *testing.T) {
			now := time.Now()
			m := &evaluationMarket{}
			id := "BTC"
			switch asset {
			case AssetSpot:
				m.snap.Quotes = []marketdata.Quote{{Symbol: id, PriceUsdt: 110, UpdatedAt: now}}
			case AssetIndex:
				id = "hsi"
				m.snap.Indices = []marketdata.IndexQuote{{ID: id, Price: 110, UpdatedAt: now}}
			case AssetAlpha:
				id = "aapl"
				m.snap.Alpha.Stocks = []marketdata.AlphaQuote{{ID: id, Price: 110, UpdatedAt: now}}
			}
			target := 100.0
			rule := Rule{ID: 1, AssetType: asset, Symbol: id, RuleType: 1, Status: StatusActive, Frequency: FrequencyLoop, Params: RuleParams{Target: &target}}
			idx := newRuleIndex()
			idx.Upsert(rule)
			q, c := &evaluationQueue{}, &evaluationCooldown{}
			e := &Evaluator{md: m, dispatcher: q, cooldown: c, index: idx, windows: NewWindowTracker(), lastSamples: map[string]time.Time{}}
			e.processSnapshotAt(now)
			e.processSnapshotAt(now.Add(time.Second))
			if len(q.rules) != 1 {
				t.Fatal("cooldown did not suppress duplicate")
			}
			c.active = false // simulate Redis TTL expiring, with exactly the same price
			e.processSnapshotAt(now.Add(2 * time.Second))
			if len(q.rules) != 2 {
				t.Fatal("stable price suppressed loop alert")
			}
			idx.Remove(rule)
			c.active = false
			e.processSnapshotAt(now.Add(3 * time.Second))
			if len(q.rules) != 2 {
				t.Fatal("disabled rule fired")
			}
			idx.Upsert(rule)
			e.processSnapshotAt(now.Add(4 * time.Second))
			if len(q.rules) != 3 {
				t.Fatal("reenabled rule was not evaluated")
			}
		})
	}
}

func TestTimerDoesNotManufactureWindowSamples(t *testing.T) {
	now := time.Now()
	m := &evaluationMarket{snap: marketdata.Snapshot{Quotes: []marketdata.Quote{{Symbol: "BTC", PriceUsdt: 100, UpdatedAt: now}}}}
	e := &Evaluator{md: m, dispatcher: &evaluationQueue{}, cooldown: &evaluationCooldown{}, index: newRuleIndex(), windows: NewWindowTracker(), lastSamples: map[string]time.Time{}}
	e.processSnapshotAt(now)
	e.processSnapshotAt(now.Add(time.Second))
	if _, ready := e.windows.SnapshotAt("spot:BTC", now.Add(time.Second)); ready {
		t.Fatal("timer fabricated second market sample")
	}
	m.snap.Quotes[0].PriceUsdt = 110
	m.snap.Quotes[0].UpdatedAt = now.Add(time.Second)
	e.processSnapshotAt(now.Add(time.Second))
	if amp, ready := e.windows.SnapshotAt("spot:BTC", now.Add(2*time.Second)); !ready || amp != 10 {
		t.Fatalf("amp=%v ready=%v", amp, ready)
	}
	if _, ready := e.windows.SnapshotAt("spot:BTC", now.Add(6*time.Minute)); ready {
		t.Fatal("expired window retained")
	}
}

type timerQueue struct{ count atomic.Int32 }

func (q *timerQueue) Enqueue(Rule, float64, triggerMeta) bool { q.count.Add(1); return true }

type timerCooldown struct{}

func (timerCooldown) TrySet(context.Context, Rule) (bool, error) { return true, nil }
func (timerCooldown) Clear(context.Context, int64) error         { return nil }

func TestEvaluatorTimerRunsWithoutMarketNotifications(t *testing.T) {
	now := time.Now()
	m := &evaluationMarket{snap: marketdata.Snapshot{Quotes: []marketdata.Quote{{Symbol: "BTC", PriceUsdt: 110, UpdatedAt: now}}}}
	idx := newRuleIndex()
	target := 100.0
	idx.Upsert(Rule{ID: 1, AssetType: AssetSpot, Symbol: "BTC", RuleType: 1, Status: StatusActive, Params: RuleParams{Target: &target}})
	q := &timerQueue{}
	e := NewEvaluator(m, q, timerCooldown{}, NewWindowTracker(), idx)
	defer e.Stop()
	e.Wake()
	deadline := time.Now().Add(3 * time.Second)
	for q.count.Load() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("unchanged quote was not evaluated on timer")
		}
		time.Sleep(time.Millisecond)
	}
	e.Stop()
	e.Stop()
}
