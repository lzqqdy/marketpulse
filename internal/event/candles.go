package event

import "github.com/lzqqdy/marketpulse/internal/marketdata/binance"

// closedCandles drops the in-progress last bar when the provider marks it open.
// If no candle is flagged Closed (unit tests / sparse fixtures), leaves the slice unchanged.
func closedCandles(candles []binance.Candle) []binance.Candle {
	if len(candles) == 0 {
		return candles
	}
	anyClosed := false
	for _, c := range candles {
		if c.Closed {
			anyClosed = true
			break
		}
	}
	if !anyClosed {
		return candles
	}
	if candles[len(candles)-1].Closed {
		return candles
	}
	return candles[:len(candles)-1]
}
