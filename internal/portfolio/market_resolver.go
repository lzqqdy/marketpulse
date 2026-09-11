package portfolio

import (
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"time"
)

type marketResolver struct {
	md marketdata.MarketDataService
}

func (m marketResolver) CryptoPrice(symbol string) PricePoint {
	q, ok := m.md.Quote(symbol)
	if !ok || q.PriceUsdt <= 0 {
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.PriceUsdt, ChangeDayPct: q.ChangeDayPct, OK: true,
		UpdatedAt: q.UpdatedAt, Stale: marketdata.IsStale(q.UpdatedAt, time.Now(), marketdata.SpotQuoteMaxAge)}
}

func (m marketResolver) AlphaPrice(id string) PricePoint {
	q, ok := m.md.AlphaQuote(id)
	if !ok || q.Price <= 0 {
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.Price, ChangeDayPct: q.ChangeDayPct, OK: true,
		UpdatedAt: q.UpdatedAt, Source: q.Source, Stale: marketdata.IsStale(q.UpdatedAt, time.Now(), marketdata.ReferenceQuoteMaxAge)}
}

func (m marketResolver) UsdtCny() (float64, float64, bool) {
	rates := m.md.Snapshot().Rates
	if rates.USDTCNY > 0 {
		return rates.USDTCNY, rates.USDCNY, !marketdata.IsStale(rates.USDTCNYUpdatedAt, time.Now(), marketdata.ConversionRateMaxAge)
	}
	return 0, rates.USDCNY, false
}
