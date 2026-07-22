package portfolio

import (
	"testing"
	"time"
)

func TestNormalizeReportRange(t *testing.T) {
	got, err := NormalizeReportRange("")
	if err != nil || got != "30d" {
		t.Fatalf("default: got %q err=%v", got, err)
	}
	got, err = NormalizeReportRange("1Y")
	if err != nil || got != "1y" {
		t.Fatalf("1y: got %q err=%v", got, err)
	}
	if _, err := NormalizeReportRange("2y"); err == nil {
		t.Fatal("expected error for 2y")
	}
}

func TestResolveReportWindow(t *testing.T) {
	now := time.Date(2026, 7, 22, 15, 0, 0, 0, time.FixedZone("CST", 8*3600))
	from, to := ResolveReportWindow("30d", now, "2021-01-01")
	if to != "2026-07-22" {
		t.Fatalf("to=%s", to)
	}
	if from != "2026-06-23" { // 30 calendar days inclusive: Jul22 - 29d = Jun23
		t.Fatalf("from=%s want 2026-06-23", from)
	}
	from, to = ResolveReportWindow("all", now, "2021-05-07")
	if from != "2021-05-07" || to != "2026-07-22" {
		t.Fatalf("all: %s..%s", from, to)
	}
}
