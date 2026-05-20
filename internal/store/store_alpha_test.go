package store

import "testing"

func TestMarketStoreAlphaSnapshot(t *testing.T) {
	s := New("BTC")
	s.SetAlphaDefaults(
		[]AlphaQuote{{ID: "qqqon", Name: "QQQ", Symbol: "QQQON", Source: "binance-alpha", Category: "index"}},
		[]AlphaQuote{{ID: "nvdaon", Name: "NVDA", Symbol: "NVDAON", Source: "binance-alpha", Category: "stock"}},
	)
	s.UpdateAlphaQuote(AlphaQuote{
		ID:           "qqqon",
		Name:         "QQQ",
		Symbol:       "QQQON",
		Price:        530.5,
		ChangeDayPct: 1.2,
		Source:       "binance-alpha",
		Category:     "index",
	})

	snap := s.GetSnapshot()
	if len(snap.Quotes) != 0 {
		t.Fatalf("alpha should not pollute crypto quotes: %+v", snap.Quotes)
	}
	if len(snap.Alpha.Indices) != 1 || snap.Alpha.Indices[0].Price != 530.5 {
		t.Fatalf("alpha indices: %+v", snap.Alpha.Indices)
	}
	if len(snap.Alpha.Stocks) != 1 || snap.Alpha.Stocks[0].Symbol != "NVDAON" {
		t.Fatalf("alpha stocks: %+v", snap.Alpha.Stocks)
	}
}
