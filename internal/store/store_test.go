package store

import (
	"sync"
	"testing"
)

func TestUpdateQuote_andSnapshot(t *testing.T) {
	s := New("BTC", "ETH")
	v := s.UpdateQuote(Quote{Symbol: "BTC", PriceUsdt: 100})
	if v != 1 {
		t.Fatalf("version: %d", v)
	}
	s.UpdateQuote(Quote{Symbol: "ETH", PriceUsdt: 10})

	snap := s.GetSnapshot()
	if len(snap.Quotes) != 2 {
		t.Fatalf("quotes: %d", len(snap.Quotes))
	}
	if snap.Quotes[0].Symbol != "BTC" || snap.Quotes[1].Symbol != "ETH" {
		t.Fatalf("order: %+v", snap.Quotes)
	}
}

func TestUpdateRates_recalculatesCny(t *testing.T) {
	s := New("BTC")
	s.UpdateQuote(Quote{Symbol: "BTC", PriceUsdt: 100})
	s.UpdateRates(Rates{USDTCNY: 7.2})

	snap := s.GetSnapshot()
	if snap.Quotes[0].PriceCny != 720 {
		t.Fatalf("cny: %f", snap.Quotes[0].PriceCny)
	}
}

func TestConcurrentUpdates(t *testing.T) {
	s := New("BTC")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.UpdateQuote(Quote{Symbol: "BTC", PriceUsdt: float64(n)})
		}(i)
	}
	wg.Wait()
	if s.Version() == 0 {
		t.Fatal("version should increase")
	}
	_ = s.GetSnapshot()
}
