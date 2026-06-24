package equity

import (
	"testing"
	"time"
)

func TestEastmoneyKlineBegEnd(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	beg, end := eastmoneyKlineBegEnd("101", 30, now)
	if end != "20260624" {
		t.Fatalf("end=%s", end)
	}
	if beg == "0" || beg >= end {
		t.Fatalf("beg=%s end=%s", beg, end)
	}
	lookback := eastmoneyKlineLookbackDays("101", 300)
	if lookback < 300 {
		t.Fatalf("daily lookback too small: %d", lookback)
	}
}

func TestEastmoneyKlineDataCandles(t *testing.T) {
	data := eastmoneyKlineData{
		Klines: []string{
			"2026-05-15,49930.26,49526.17,49930.26,49503.57,588000000,0.00,0.85,-1.07,-537.29,0.00",
			"2026-05-18,4547.6,4537.5,4559.0,4483.5,27964,0.0,1.66,-0.53,-24.4,0.00",
		},
	}
	candles, err := data.candles()
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("len=%d", len(candles))
	}
	if candles[0].Open != 49930.26 || candles[1].Close != 4537.5 {
		t.Fatalf("candles=%+v", candles)
	}
}

func TestNormalizeEastmoneyKlineInterval(t *testing.T) {
	klt, err := normalizeEastmoneyKlineInterval("1w")
	if err != nil {
		t.Fatal(err)
	}
	if klt != "102" {
		t.Fatalf("klt=%s", klt)
	}
}

func TestKlineLimitAttempts(t *testing.T) {
	got := klineLimitAttempts(300)
	want := []int{300, 120, 60, 30}
	if len(got) != len(want) {
		t.Fatalf("len = %d, got %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempts = %+v, want %+v", got, want)
		}
	}

	got = klineLimitAttempts(60)
	want = []int{60, 30}
	if len(got) != len(want) {
		t.Fatalf("len = %d, got %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempts = %+v, want %+v", got, want)
		}
	}
}
