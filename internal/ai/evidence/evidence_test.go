package evidence

import (
	"testing"
	"time"
)

func TestIdentityAndExpiry(t *testing.T) {
	now := time.Now()
	e := Evidence{Kind: "quote", InstrumentID: "us-aapl", Source: "tencent", AsOf: now, Status: OK, FetchedAt: now, ExpiresAt: now.Add(time.Minute), Facts: map[string]any{"price": 200}}
	e.Seal()
	copy := e
	copy.FetchedAt = now.Add(time.Second)
	copy.ExpiresAt = now.Add(2 * time.Minute)
	copy.Seal()
	if e.ID != copy.ID {
		t.Fatal("retrieval time changed fact identity")
	}
	if e.Revalidate(now).Status != OK || e.Revalidate(now.Add(time.Minute)).Status != Stale {
		t.Fatal("wrong expiry boundary")
	}
	copy = e
	copy.InstrumentID = "bitget:AAPL"
	copy.Seal()
	if e.ID == copy.ID {
		t.Fatal("different instruments share evidence identity")
	}
}
