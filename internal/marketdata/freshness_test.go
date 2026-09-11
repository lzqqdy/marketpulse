package marketdata

import (
	"testing"
	"time"
)

func TestFreshness(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		at    time.Time
		stale bool
	}{
		{now, false}, {now.Add(-SpotQuoteMaxAge), false}, {now.Add(-SpotQuoteMaxAge - time.Second), true}, {time.Time{}, true}, {now.Add(2 * time.Minute), true},
	} {
		if got := IsStale(tc.at, now, SpotQuoteMaxAge); got != tc.stale {
			t.Fatalf("at=%v stale=%v", tc.at, got)
		}
	}
}
