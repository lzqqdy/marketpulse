package event

import (
	"sync"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
)

// LiquidationTracker keeps a ring of totalUsd samples for baseline ratio.
type LiquidationTracker struct {
	mu       sync.Mutex
	samples  []float64
	max      int
	lastAdd  time.Time
	minGap   time.Duration
	ratioThr float64
}

// NewLiquidationTracker builds a tracker from config.
func NewLiquidationTracker(cfg config.EventLiquidationConfig) *LiquidationTracker {
	max := cfg.BaselineSamples
	if max <= 0 {
		max = 60
	}
	gap := cfg.SampleInterval
	if gap <= 0 {
		gap = time.Minute
	}
	ratio := cfg.Ratio
	if ratio <= 0 {
		ratio = 3
	}
	return &LiquidationTracker{max: max, minGap: gap, ratioThr: ratio}
}

// Observe records totalUsd (rate-limited) and maybe emits LIQUIDATION_SPIKE.
func (t *LiquidationTracker) Observe(totalUsd float64, now time.Time) []EventSignal {
	t.mu.Lock()
	defer t.mu.Unlock()
	if totalUsd <= 0 {
		return nil
	}
	if !t.lastAdd.IsZero() && now.Sub(t.lastAdd) < t.minGap {
		// Still evaluate against existing baseline without appending.
	} else {
		t.samples = append(t.samples, totalUsd)
		if len(t.samples) > t.max {
			t.samples = t.samples[len(t.samples)-t.max:]
		}
		t.lastAdd = now
	}
	if len(t.samples) < 5 {
		return nil
	}
	// Baseline excludes current sample when possible.
	n := len(t.samples) - 1
	if n < 4 {
		return nil
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += t.samples[i]
	}
	baseline := sum / float64(n)
	if baseline <= 0 {
		return nil
	}
	ratio := totalUsd / baseline
	if ratio < t.ratioThr {
		return nil
	}
	return []EventSignal{{
		ID:         newSignalID(),
		SignalType: SignalLiquidationSpike,
		Symbol:     SymbolMarketWide,
		Market:     MarketCrypto,
		Value:      totalUsd,
		Baseline:   baseline,
		Ratio:      ratio,
		Window:     "1h",
		Direction:  DirectionUp,
		Timestamp:  now.UTC(),
		Metadata: map[string]any{
			"note": "exchange_wide_1h_window",
		},
	}}
}
