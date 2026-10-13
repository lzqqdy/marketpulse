package store

import (
	"sync"
	"testing"
	"time"
)

func TestRateFeedsMergeAtomically(t *testing.T) {
	s := New("BTC")
	s.UpdateQuote(Quote{Symbol: "BTC", PriceUsdt: 100})
	otcAt := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	fxAt := otcAt.Add(time.Second)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); <-start; s.UpdateUSDTCNY(7.2, otcAt) }()
		go func() { defer wg.Done(); <-start; s.UpdateUSDCNY(7.1, fxAt) }()
	}
	close(start)
	wg.Wait()
	snap := s.GetSnapshot()
	if snap.Rates.USDTCNY != 7.2 || snap.Rates.USDCNY != 7.1 || !snap.Rates.USDTCNYUpdatedAt.Equal(otcAt) || !snap.Rates.USDCNYUpdatedAt.Equal(fxAt) {
		t.Fatalf("lost feed update: %+v", snap.Rates)
	}
	if !snap.Rates.UpdatedAt.Equal(fxAt) || snap.Quotes[0].PriceCny != 720 {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestOlderRateResponseCannotRevertNewRate(t *testing.T) {
	s := New()
	now := time.Now()
	s.UpdateUSDTCNY(7.2, now)
	s.UpdateUSDCNY(7.1, now)
	version := s.Version()
	s.UpdateUSDTCNY(6, now.Add(-time.Minute))
	s.UpdateUSDCNY(5, now.Add(-time.Minute))
	r := s.GetSnapshot().Rates
	if s.Version() != version || r.USDTCNY != 7.2 || r.USDCNY != 7.1 {
		t.Fatalf("stale update accepted: %+v", r)
	}
}
