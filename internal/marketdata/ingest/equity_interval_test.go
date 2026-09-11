package ingest

import (
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
	"testing"
	"time"
)

func TestConfiguredQuoteTTLAndPolling(t *testing.T) {
	cfg := &config.Config{}
	cfg.Ingest.Equity.Interval = 30 * time.Second
	s := &Service{cfg: cfg}
	def := equity.IndexDef{ID: "sh000001"}
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	if got := s.equityQuoteTTL(def, now); got != 30*time.Second {
		t.Fatal(got)
	}
	if got := equity.NextPollInterval([]equity.IndexDef{def}, now, cfg.Ingest.Equity.Interval); got != 30*time.Second {
		t.Fatal(got)
	}
	if got := s.equityQuoteTTL(def, now.AddDate(0, 0, 1)); got != time.Hour {
		t.Fatal(got)
	}
	cache := newEquityCache()
	cache.merge(map[string]store.IndexQuote{def.ID: {ID: def.ID, Price: 100}}, now)
	if got := cache.expiredDefs([]equity.IndexDef{def}, now.Add(29*time.Second), s.equityQuoteTTL); len(got) != 0 {
		t.Fatal("expired too soon")
	}
	if got := cache.expiredDefs([]equity.IndexDef{def}, now.Add(30*time.Second), s.equityQuoteTTL); len(got) != 1 {
		t.Fatal("30s cache still suppresses refresh")
	}
}
