package equity

import (
	"testing"
	"time"
)

func TestCacheTTL(t *testing.T) {
	weekday := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) // Friday
	if got := CacheTTL(weekday); got != time.Minute {
		t.Fatalf("weekday ttl=%v", got)
	}

	weekend := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC) // Sunday
	if got := CacheTTL(weekend); got != time.Hour {
		t.Fatalf("weekend ttl=%v", got)
	}
}
