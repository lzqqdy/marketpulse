package binance

import (
	"testing"
	"time"
)

func TestDayStartShanghai(t *testing.T) {
	// 2026-05-16 20:12 CST = 2026-05-16 12:12 UTC
	utc := time.Date(2026, 5, 16, 12, 12, 0, 0, time.UTC)
	start := DayStartShanghai(utc)
	if start.Hour() != 0 || start.Minute() != 0 {
		t.Fatalf("hour/min: %v", start)
	}
	if DayKeyShanghai(utc) != "2026-05-16" {
		t.Fatalf("key: %s", DayKeyShanghai(utc))
	}
	// midnight boundary: 00:01 CST same day
	cst := time.Date(2026, 5, 16, 0, 1, 0, 0, Shanghai)
	if DayKeyShanghai(cst) != "2026-05-16" {
		t.Fatal()
	}
}

func TestNextDayStartShanghai(t *testing.T) {
	utc := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	next := NextDayStartShanghai(utc)
	if DayKeyShanghai(next) != "2026-05-17" {
		t.Fatalf("next day key: %s", DayKeyShanghai(next))
	}
}
