package binance

import (
	"testing"
	"time"
)

func TestShanghaiDayStartUTC(t *testing.T) {
	cst := time.Date(2026, 5, 17, 0, 2, 0, 0, Shanghai)
	start := ShanghaiDayStartUTC(cst)
	if !start.Equal(time.Date(2026, 5, 16, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("start: %v", start)
	}
	if ShanghaiDayKey(cst) != "2026-05-17" {
		t.Fatalf("key: %s", ShanghaiDayKey(cst))
	}

	beforeMidnight := time.Date(2026, 5, 16, 23, 59, 0, 0, Shanghai)
	if ShanghaiDayKey(beforeMidnight) != "2026-05-16" {
		t.Fatalf("key before midnight: %s", ShanghaiDayKey(beforeMidnight))
	}
}

func TestNextShanghaiDayStartUTC(t *testing.T) {
	cst := time.Date(2026, 5, 17, 0, 2, 0, 0, Shanghai)
	next := NextShanghaiDayStartUTC(cst)
	if !next.In(Shanghai).Equal(time.Date(2026, 5, 18, 0, 0, 0, 0, Shanghai)) {
		t.Fatalf("next start: %v", next.In(Shanghai))
	}
}
