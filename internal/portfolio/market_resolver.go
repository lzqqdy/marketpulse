package portfolio

import (
	"strings"

	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

type marketResolver struct {
	md marketdata.MarketDataService
}

func (m marketResolver) CryptoPrice(symbol string) PricePoint {
	q, ok := m.md.Quote(symbol)
	if !ok || q.PriceUsdt <= 0 {
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.PriceUsdt, ChangeDayPct: q.ChangeDayPct, OK: true}
}

func (m marketResolver) AlphaPrice(id string) PricePoint {
	q, ok := m.md.AlphaQuote(id)
	if !ok || q.Price <= 0 {
		// try matching by trading symbol as fallback
		snap := m.md.Snapshot().Alpha
		for _, row := range append(append([]marketdata.AlphaQuote{}, snap.Indices...), snap.Stocks...) {
			if strings.EqualFold(row.Symbol, id) || strings.EqualFold(row.ID, id) {
				if row.Price > 0 {
					return PricePoint{PriceUsdt: row.Price, ChangeDayPct: row.ChangeDayPct, OK: true}
				}
			}
		}
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.Price, ChangeDayPct: q.ChangeDayPct, OK: true}
}

func (m marketResolver) UsdtCny() (float64, float64, bool) {
	rates := m.md.Snapshot().Rates
	if rates.USDTCNY > 0 {
		return rates.USDTCNY, rates.USDCNY, true
	}
	return 0, rates.USDCNY, false
}
