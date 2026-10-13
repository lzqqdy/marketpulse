package alerts

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

// Evaluator watches market data changes and triggers alert rules.
type Evaluator struct {
	md          marketdata.MarketDataService
	dispatcher  alertEnqueuer
	cooldown    cooldownClaimer
	windows     *WindowTracker
	index       *ruleIndex
	lastSamples map[string]time.Time
	ch          chan uint64
	stop        chan struct{}
	stopOnce    sync.Once
}

type alertEnqueuer interface {
	Enqueue(Rule, float64, triggerMeta) bool
}
type cooldownClaimer interface {
	TrySet(context.Context, Rule) (bool, error)
	Clear(context.Context, int64) error
}

type ruleIndex struct {
	mu    sync.RWMutex
	rules map[string][]Rule
}

func newRuleIndex() *ruleIndex {
	return &ruleIndex{rules: make(map[string][]Rule)}
}

func indexKey(assetType, symbol string) string {
	return assetType + ":" + symbol
}

func (idx *ruleIndex) Rebuild(rules []Rule) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.rules = make(map[string][]Rule)
	for _, r := range rules {
		key := indexKey(r.AssetType, normalizeIndexSymbol(r.AssetType, r.Symbol))
		idx.rules[key] = append(idx.rules[key], r)
	}
}

func (idx *ruleIndex) Upsert(rule Rule) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	key := indexKey(rule.AssetType, normalizeIndexSymbol(rule.AssetType, rule.Symbol))
	for i, r := range idx.rules[key] {
		if r.ID == rule.ID {
			idx.rules[key][i] = rule
			return
		}
	}
	idx.rules[key] = append(idx.rules[key], rule)
}

func (idx *ruleIndex) Remove(rule Rule) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	key := indexKey(rule.AssetType, normalizeIndexSymbol(rule.AssetType, rule.Symbol))
	list := idx.rules[key]
	out := list[:0]
	for _, r := range list {
		if r.ID != rule.ID {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		delete(idx.rules, key)
	} else {
		idx.rules[key] = out
	}
}

func (idx *ruleIndex) Get(assetType, symbol string) []Rule {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if assetType != AssetAlpha {
		return append([]Rule(nil), idx.rules[indexKey(assetType, symbol)]...)
	}
	seen := map[int64]struct{}{}
	out := make([]Rule, 0)
	for _, alias := range marketdata.AlphaIDAliases(symbol) {
		for _, r := range idx.rules[indexKey(assetType, alias)] {
			if _, ok := seen[r.ID]; ok {
				continue
			}
			seen[r.ID] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func NewEvaluator(
	md marketdata.MarketDataService,
	dispatcher alertEnqueuer,
	cooldown cooldownClaimer,
	windows *WindowTracker,
	index *ruleIndex,
) *Evaluator {
	e := &Evaluator{
		md:          md,
		dispatcher:  dispatcher,
		cooldown:    cooldown,
		windows:     windows,
		index:       index,
		lastSamples: make(map[string]time.Time),
		ch:          make(chan uint64, 256),
		stop:        make(chan struct{}),
	}
	md.AddListener(e.onStoreChange)
	go e.run()
	return e
}

func (e *Evaluator) Stop() {
	e.stopOnce.Do(func() { close(e.stop) })
}

func (e *Evaluator) Wake() { e.onStoreChange(0) }

func (e *Evaluator) onStoreChange(version uint64) {
	select {
	case e.ch <- version:
	default:
	}
}

func (e *Evaluator) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-e.ch:
			e.processSnapshot()
		case <-ticker.C:
			e.processSnapshot()
		}
	}
}

func (e *Evaluator) processSnapshot() {
	e.processSnapshotAt(time.Now())
}

func (e *Evaluator) sample(key string, price float64, sampledAt, now time.Time) (float64, bool) {
	if !sampledAt.IsZero() && sampledAt.After(e.lastSamples[key]) {
		e.lastSamples[key] = sampledAt
		return e.windows.Update(key, price, sampledAt)
	}
	return e.windows.SnapshotAt(key, now)
}

func (e *Evaluator) processSnapshotAt(now time.Time) {
	snap := e.md.Snapshot()

	for _, q := range snap.Quotes {
		if q.PriceUsdt <= 0 || marketdata.IsStale(q.UpdatedAt, now, marketdata.SpotQuoteMaxAge) {
			continue
		}
		sym := strings.ToUpper(strings.TrimSpace(q.Symbol))
		key := indexKey(AssetSpot, sym)
		amp, ready := e.sample(key, q.PriceUsdt, q.UpdatedAt, now)
		e.evaluateAsset(AssetSpot, sym, q.PriceUsdt, amp, ready)
	}

	for _, idx := range snap.Indices {
		if idx.Price <= 0 || idx.Stale {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(idx.ID))
		key := indexKey(AssetIndex, id)
		at := idx.FetchedAt
		if at.IsZero() {
			at = idx.UpdatedAt
		}
		amp, ready := e.sample(key, idx.Price, at, now)
		e.evaluateAsset(AssetIndex, id, idx.Price, amp, ready)
	}

	alphaRows := append(append([]marketdata.AlphaQuote(nil), snap.Alpha.Indices...), snap.Alpha.Stocks...)
	for _, row := range alphaRows {
		if row.Price <= 0 || marketdata.IsStale(row.UpdatedAt, now, marketdata.ReferenceQuoteMaxAge) {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(row.ID))
		if id == "" {
			continue
		}
		key := indexKey(AssetAlpha, id)
		amp, ready := e.sample(key, row.Price, row.UpdatedAt, now)
		e.evaluateAsset(AssetAlpha, id, row.Price, amp, ready)
	}
}

func (e *Evaluator) evaluateAsset(assetType, symbol string, price, amp float64, windowReady bool) {
	ctx := context.Background()
	meta := e.triggerMetaFor(assetType, symbol, price, amp)
	for _, rule := range e.index.Get(assetType, symbol) {
		if rule.Status != StatusActive {
			continue
		}
		met, triggerVal := IsConditionMet(rule.RuleType, price, rule.Params, amp, windowReady)
		if !met {
			continue
		}
		claimed, err := e.cooldown.TrySet(ctx, rule)
		if err != nil {
			slog.Warn("alerts cooldown claim", "rule_id", rule.ID, "err", err)
			continue
		}
		if !claimed {
			continue
		}
		jobMeta := meta
		if rule.RuleType == 5 {
			jobMeta.WindowAmp = triggerVal
		}
		if !e.dispatcher.Enqueue(rule, triggerVal, jobMeta) {
			_ = e.cooldown.Clear(ctx, rule.ID)
		}
	}
}

func (e *Evaluator) triggerMetaFor(assetType, symbol string, price, amp float64) triggerMeta {
	meta := triggerMeta{Price: price, WindowAmp: amp}
	switch assetType {
	case AssetSpot:
		base := normalizeSpotBase(symbol)
		if q, ok := e.md.Quote(base); ok {
			meta.Price = q.PriceUsdt
			meta.ChangePct = q.ChangeDayPct
			meta.DisplayName = displayName(AssetSpot, q.Symbol, "")
			return meta
		}
	case AssetIndex:
		if q, ok := e.md.IndexQuote(symbol); ok {
			if q.Price > 0 {
				meta.Price = q.Price
			}
			meta.ChangePct = q.ChangePct
			meta.DisplayName = displayName(AssetIndex, q.ID, q.Name)
			return meta
		}
	case AssetAlpha:
		if q, ok := e.md.AlphaQuote(symbol); ok {
			if q.Price > 0 {
				meta.Price = q.Price
			}
			meta.ChangePct = q.ChangeDayPct
			if meta.ChangePct == 0 {
				meta.ChangePct = q.Change24hPct
			}
			live := strings.TrimSpace(q.Name)
			if live == "" {
				live = q.Symbol
			}
			meta.DisplayName = displayName(AssetAlpha, q.ID, live)
			return meta
		}
	}
	meta.DisplayName = displayName(assetType, symbol, "")
	return meta
}

func (e *Evaluator) WindowAmplitude(assetType, symbol string) (float64, bool) {
	key := indexKey(assetType, normalizeIndexSymbol(assetType, symbol))
	return e.windows.Snapshot(key)
}

func normalizeIndexSymbol(assetType, symbol string) string {
	if assetType == AssetIndex || assetType == AssetAlpha {
		return strings.ToLower(strings.TrimSpace(symbol))
	}
	return normalizeSpotBase(symbol)
}
