package portfolio

import (
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"time"
)

type marketResolver struct {
	md interface {
		Quote(string) (marketdata.Quote, bool)
		AlphaQuote(string) (marketdata.AlphaQuote, bool)
		Snapshot() marketdata.Snapshot
	}
	at time.Time
}

func (m marketResolver) now() time.Time {
	if !m.at.IsZero() {
		return m.at
	}
	return time.Now()
}

type frozenMarket struct{ snap marketdata.Snapshot }

func (m frozenMarket) Snapshot() marketdata.Snapshot { return m.snap }
func (m frozenMarket) Quote(symbol string) (marketdata.Quote, bool) {
	for _, q := range m.snap.Quotes {
		if q.Symbol == symbol {
			return q, true
		}
	}
	return marketdata.Quote{}, false
}
func (m frozenMarket) AlphaQuote(id string) (marketdata.AlphaQuote, bool) {
	return marketdata.FindAlphaQuote(m.snap.Alpha.Indices, m.snap.Alpha.Stocks, id)
}

func (m marketResolver) CryptoPrice(symbol string) PricePoint {
	q, ok := m.md.Quote(symbol)
	if !ok || q.PriceUsdt <= 0 {
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.PriceUsdt, ChangeDayPct: q.ChangeDayPct, OK: true,
		UpdatedAt: q.UpdatedAt, Stale: marketdata.IsStale(q.UpdatedAt, m.now(), marketdata.SpotQuoteMaxAge)}
}

func (m marketResolver) AlphaPrice(id string) PricePoint {
	q, ok := m.md.AlphaQuote(id)
	if !ok || q.Price <= 0 {
		return PricePoint{}
	}
	return PricePoint{PriceUsdt: q.Price, ChangeDayPct: q.ChangeDayPct, OK: true,
		UpdatedAt: q.UpdatedAt, Source: q.Source, Stale: marketdata.IsStale(q.UpdatedAt, m.now(), marketdata.ReferenceQuoteMaxAge)}
}

func (m marketResolver) UsdtCny() (float64, float64, bool) {
	rates := m.md.Snapshot().Rates
	if rates.USDTCNY > 0 {
		return rates.USDTCNY, rates.USDCNY, !marketdata.IsStale(rates.USDTCNYUpdatedAt, m.now(), marketdata.ConversionRateMaxAge)
	}
	return 0, rates.USDCNY, false
}
