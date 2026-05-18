package store

import "testing"

func TestUpdateQuoteKeepDayPct(t *testing.T) {
	s := New("BTC")
	s.UpdateQuote(Quote{Symbol: "BTC", PriceUsdt: 100, ChangeDayPct: 1.5, Change24hPct: -0.5})

	s.UpdateQuoteKeepDayPct(Quote{Symbol: "BTC", PriceUsdt: 101, Change24hPct: -0.4})
	snap := s.GetSnapshot()
	if snap.Quotes[0].ChangeDayPct != 1.5 {
		t.Fatalf("day pct preserved: got %v", snap.Quotes[0].ChangeDayPct)
	}
	if snap.Quotes[0].PriceUsdt != 101 {
		t.Fatalf("price updated: got %v", snap.Quotes[0].PriceUsdt)
	}
}
