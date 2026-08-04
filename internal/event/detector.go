package event

import (
	"math"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

// DetectPrice emits PRICE_DROP / PRICE_SPIKE from candles for configured windows.
func DetectPrice(symbol string, candles []binance.Candle, cfg config.EventPriceConfig, now time.Time) []EventSignal {
	candles = closedCandles(candles)
	out := make([]EventSignal, 0)
	for window, thr := range cfg {
		need := barsForWindow(window)
		if need <= 0 || len(candles) < need+1 {
			continue
		}
		// Last closed bar vs close need bars ago.
		cur := candles[len(candles)-1]
		prev := candles[len(candles)-1-need]
		if prev.Close <= 0 {
			continue
		}
		changePct := (cur.Close - prev.Close) / prev.Close * 100
		threshold := thr.ThresholdPct
		if threshold <= 0 {
			continue
		}
		if math.Abs(changePct) < threshold {
			continue
		}
		sigType := SignalPriceSpike
		dir := DirectionUp
		if changePct < 0 {
			sigType = SignalPriceDrop
			dir = DirectionDown
		}
		out = append(out, EventSignal{
			ID:         newSignalID(),
			SignalType: sigType,
			Symbol:     symbol,
			Market:     MarketCrypto,
			Value:      changePct,
			Baseline:   threshold,
			ChangePct:  changePct,
			Window:     window,
			Direction:  dir,
			Timestamp:  now.UTC(),
			Metadata:   map[string]any{"interval": window, "bars": need},
		})
	}
	return out
}

// DetectVolume emits VOLUME_SPIKE when latest closed bar volume / lookback mean exceeds ratio.
func DetectVolume(symbol, window string, candles []binance.Candle, cfg config.EventVolumeConfig, now time.Time) []EventSignal {
	candles = closedCandles(candles)
	lookback := cfg.LookbackBars
	if lookback <= 0 {
		lookback = 20
	}
	if len(candles) < lookback+1 {
		return nil
	}
	cur := candles[len(candles)-1]
	var sum float64
	for i := len(candles) - 1 - lookback; i < len(candles)-1; i++ {
		sum += candles[i].Volume
	}
	baseline := sum / float64(lookback)
	if baseline <= 0 {
		return nil
	}
	ratio := cur.Volume / baseline
	minRatio := cfg.RatioMedium
	if minRatio <= 0 {
		minRatio = 2
	}
	if ratio < minRatio {
		return nil
	}
	return []EventSignal{{
		ID:         newSignalID(),
		SignalType: SignalVolumeSpike,
		Symbol:     symbol,
		Market:     MarketCrypto,
		Value:      cur.Volume,
		Baseline:   baseline,
		Ratio:      ratio,
		Window:     window,
		Direction:  DirectionUp,
		Timestamp:  now.UTC(),
		Metadata:   map[string]any{"lookbackBars": lookback},
	}}
}

// DetectVolatility emits VOLATILITY_SPIKE when current ATR exceeds lookback mean ATR * ratio.
func DetectVolatility(symbol, window string, candles []binance.Candle, cfg config.EventVolatilityConfig, now time.Time) []EventSignal {
	candles = closedCandles(candles)
	lookback := cfg.LookbackBars
	if lookback <= 0 {
		lookback = 20
	}
	ratioThr := cfg.Ratio
	if ratioThr <= 0 {
		ratioThr = 2
	}
	if len(candles) < lookback+2 {
		return nil
	}
	atrs := make([]float64, 0, len(candles))
	for i := 1; i < len(candles); i++ {
		atrs = append(atrs, trueRange(candles[i], candles[i-1].Close))
	}
	if len(atrs) < lookback+1 {
		return nil
	}
	cur := atrs[len(atrs)-1]
	var sum float64
	for i := len(atrs) - 1 - lookback; i < len(atrs)-1; i++ {
		sum += atrs[i]
	}
	baseline := sum / float64(lookback)
	if baseline <= 0 {
		return nil
	}
	ratio := cur / baseline
	if ratio < ratioThr {
		return nil
	}
	return []EventSignal{{
		ID:         newSignalID(),
		SignalType: SignalVolatilitySpike,
		Symbol:     symbol,
		Market:     MarketCrypto,
		Value:      cur,
		Baseline:   baseline,
		Ratio:      ratio,
		Window:     window,
		Direction:  DirectionUp,
		Timestamp:  now.UTC(),
		Metadata:   map[string]any{"lookbackBars": lookback},
	}}
}

func trueRange(c binance.Candle, prevClose float64) float64 {
	a := c.High - c.Low
	b := math.Abs(c.High - prevClose)
	d := math.Abs(c.Low - prevClose)
	return math.Max(a, math.Max(b, d))
}

func barsForWindow(window string) int {
	switch window {
	case "1m":
		return 1
	case "5m":
		return 1 // when using 5m candles, 1 bar = 5m
	case "15m":
		return 1
	case "30m":
		return 1
	case "1h":
		return 1
	case "4h":
		return 1
	case "1d":
		return 1
	default:
		return 0
	}
}
