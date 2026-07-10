package equity

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const flatChangeThreshold = 0.05

// AShareRow is a normalized A-share realtime quote row.
type AShareRow struct {
	Code            string
	Name            string
	Price           float64
	ChangePct       float64
	Amount          float64
	Volume          float64
	TurnoverRate    float64
	MarketCap       float64
	FloatMarketCap  float64
}

// ComputeMarketBreadth derives market width metrics from A-share rows.
func ComputeMarketBreadth(rows []AShareRow, now time.Time, source string) store.MarketBreadth {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	total := len(rows)
	b := store.MarketBreadth{
		Total:     total,
		UpdatedAt: now,
		Source:    source,
	}
	if total == 0 {
		return b
	}

	changes := make([]float64, 0, total)
	var amountUp, amountTotal float64
	for _, row := range rows {
		pct := row.ChangePct
		changes = append(changes, pct)
		if row.Amount > 0 {
			amountTotal += row.Amount
		}
		switch {
		case pct > flatChangeThreshold:
			b.Up++
			amountUp += row.Amount
			if isLimitUp(row.Code, row.Name, pct) {
				b.LimitUp++
			}
		case pct < -flatChangeThreshold:
			b.Down++
			if isLimitDown(row.Code, row.Name, pct) {
				b.LimitDown++
			}
		default:
			b.Flat++
		}
	}

	if total > 0 {
		b.UpPct = float64(b.Up) / float64(total) * 100
		b.DownPct = float64(b.Down) / float64(total) * 100
	}
	if b.Down > 0 {
		b.AdvanceDeclineRatio = float64(b.Up) / float64(b.Down)
	} else if b.Up > 0 {
		b.AdvanceDeclineRatio = float64(b.Up)
	}
	if amountTotal > 0 {
		b.UpTurnoverPct = amountUp / amountTotal * 100
	}

	sort.Float64s(changes)
	b.MedianChangePct = medianFloat64(changes)
	b.EqualWeightChangePct = meanFloat64(changes)
	return b
}

func limitMoveThreshold(code, name string) float64 {
	upperName := strings.ToUpper(name)
	if strings.Contains(upperName, "ST") {
		return 4.8
	}
	code = strings.TrimSpace(code)
	switch {
	case strings.HasPrefix(code, "688"), strings.HasPrefix(code, "300"), strings.HasPrefix(code, "301"):
		return 19.5
	case strings.HasPrefix(code, "8"), strings.HasPrefix(code, "4"):
		return 29.5
	default:
		return 9.5
	}
}

func isLimitUp(code, name string, changePct float64) bool {
	return changePct >= limitMoveThreshold(code, name)
}

func isLimitDown(code, name string, changePct float64) bool {
	return changePct <= -limitMoveThreshold(code, name)
}

func medianFloat64(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return values[n/2]
	}
	return (values[n/2-1] + values[n/2]) / 2
}

func meanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func roundPct(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}
