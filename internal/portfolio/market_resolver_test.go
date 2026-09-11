package portfolio

import (
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
	"testing"
	"time"
)

type freshnessMarket struct {
	marketdata.MarketDataService
	quote marketdata.Quote
	alpha marketdata.AlphaQuote
	rates store.Rates
}

func (m freshnessMarket) Quote(string) (marketdata.Quote, bool)           { return m.quote, true }
func (m freshnessMarket) AlphaQuote(string) (marketdata.AlphaQuote, bool) { return m.alpha, true }
func (m freshnessMarket) Snapshot() marketdata.Snapshot                   { return marketdata.Snapshot{Rates: m.rates} }

func TestResolverUsesQuoteAndIndividualRateTimestamps(t *testing.T) {
	now := time.Now()
	m := freshnessMarket{
		quote: marketdata.Quote{PriceUsdt: 100, UpdatedAt: now.Add(-time.Hour)},
		alpha: marketdata.AlphaQuote{Price: 200, UpdatedAt: now, Source: "bitget"},
		rates: store.Rates{USDTCNY: 7, USDCNY: 7, UpdatedAt: now, USDTCNYUpdatedAt: now.Add(-time.Hour)},
	}
	r := marketResolver{md: m}
	if p := r.CryptoPrice("BTC"); !p.OK || !p.Stale || p.PriceUsdt != 100 {
		t.Fatalf("%+v", p)
	}
	if p := r.AlphaPrice("aapl"); !p.OK || p.Stale || p.Source != "bitget" {
		t.Fatalf("%+v", p)
	}
	if rate, _, ok := r.UsdtCny(); rate != 7 || ok {
		t.Fatal("forex update masked stale OTC rate")
	}
}
