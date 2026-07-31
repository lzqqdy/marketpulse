package event

import "math"

// Default component weights (normalized over present components).
const (
	weightPrice       = 0.35
	weightVolume      = 0.25
	weightVolatility  = 0.15
	weightLiquidation = 0.25
)

// ComputeScore returns 0–100 score from present component scores.
func ComputeScore(c ComponentScores) float64 {
	type part struct {
		score  float64
		weight float64
	}
	parts := make([]part, 0, 4)
	if c.Price != nil {
		parts = append(parts, part{clamp100(*c.Price), weightPrice})
	}
	if c.Volume != nil {
		parts = append(parts, part{clamp100(*c.Volume), weightVolume})
	}
	if c.Volatility != nil {
		parts = append(parts, part{clamp100(*c.Volatility), weightVolatility})
	}
	if c.Liquidation != nil {
		parts = append(parts, part{clamp100(*c.Liquidation), weightLiquidation})
	}
	if len(parts) == 0 {
		return 0
	}
	var sumW, sum float64
	for _, p := range parts {
		sumW += p.weight
		sum += p.score * p.weight
	}
	if sumW <= 0 {
		return 0
	}
	return clamp100(sum / sumW)
}

// SeverityFromScore maps score to severity label.
func SeverityFromScore(score float64) string {
	s := clamp100(score)
	switch {
	case s < 30:
		return SeverityNormal
	case s < 50:
		return SeverityLow
	case s < 70:
		return SeverityMedium
	case s < 85:
		return SeverityHigh
	default:
		return SeverityExtreme
	}
}

// ScoreFromSignals derives component scores from signal list then aggregates.
func ScoreFromSignals(signals []EventSignal) (float64, string) {
	var c ComponentScores
	for _, sig := range signals {
		sc := signalComponentScore(sig)
		switch sig.SignalType {
		case SignalPriceDrop, SignalPriceSpike:
			c.Price = maxPtr(c.Price, sc)
		case SignalVolumeSpike:
			c.Volume = maxPtr(c.Volume, sc)
		case SignalVolatilitySpike:
			c.Volatility = maxPtr(c.Volatility, sc)
		case SignalLiquidationSpike:
			c.Liquidation = maxPtr(c.Liquidation, sc)
		}
	}
	score := ComputeScore(c)
	return score, SeverityFromScore(score)
}

func signalComponentScore(sig EventSignal) float64 {
	switch sig.SignalType {
	case SignalPriceDrop, SignalPriceSpike:
		pct := math.Abs(sig.ChangePct)
		// 2% → ~50, 5% → ~80, 10%+ → 100
		return clamp100(pct / 5 * 80)
	case SignalVolumeSpike, SignalLiquidationSpike:
		if sig.Ratio <= 0 {
			return 0
		}
		// ratio 2 → 40, 3 → 60, 5 → 85, 9 → 100
		return clamp100((sig.Ratio - 1) / 8 * 100)
	case SignalVolatilitySpike:
		if sig.Ratio <= 0 {
			return 50
		}
		return clamp100(sig.Ratio / 3 * 80)
	default:
		return 0
	}
}

func clamp100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func maxPtr(cur *float64, v float64) *float64 {
	if cur == nil || v > *cur {
		x := v
		return &x
	}
	return cur
}
