package event

import (
	"testing"
	"time"
)

func TestNextStatusTransitions(t *testing.T) {
	if got := NextStatus(StatusDetected, 70, 70, true); got != StatusActive {
		t.Fatalf("detected→active got %s", got)
	}
	if got := NextStatus(StatusActive, 40, 80, true); got != StatusDeescalating {
		t.Fatalf("active→deescalating got %s", got)
	}
	if got := NextStatus(StatusActive, 20, 80, false); got != StatusResolved {
		t.Fatalf("active→resolved got %s", got)
	}
	if got := NextStatus(StatusDeescalating, 75, 80, true); got != StatusActive {
		t.Fatalf("deescalating→active got %s", got)
	}
	if got := NextStatus(StatusDeescalating, 10, 80, false); got != StatusResolved {
		t.Fatalf("deescalating→resolved got %s", got)
	}
}

func TestApplyLifecyclePeakAndEnd(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	ev := &MarketEvent{Status: StatusDetected, PeakScore: 0}
	ApplyLifecycle(ev, 70, now)
	if ev.Status != StatusActive || ev.PeakScore != 70 {
		t.Fatalf("unexpected after first apply: %+v", ev)
	}
	ApplyLifecycle(ev, 90, now.Add(time.Minute))
	if ev.PeakScore != 90 {
		t.Fatalf("peak not updated: %v", ev.PeakScore)
	}
	ApplyLifecycle(ev, 20, now.Add(2*time.Minute))
	if ev.Status != StatusResolved || ev.EndTime == nil {
		t.Fatalf("expected resolved with endTime, got status=%s end=%v", ev.Status, ev.EndTime)
	}
}

func TestShouldForceResolve(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	ev := &MarketEvent{
		Status:    StatusDeescalating,
		StartTime: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-30 * time.Minute),
	}
	if !ShouldForceResolve(ev, now, 90*time.Minute, 20*time.Minute) {
		t.Fatal("expected force resolve by max open age")
	}
	ev2 := &MarketEvent{
		Status:    StatusDeescalating,
		StartTime: now.Add(-10 * time.Minute),
		UpdatedAt: now.Add(-25 * time.Minute),
	}
	if !ShouldForceResolve(ev2, now, 90*time.Minute, 20*time.Minute) {
		t.Fatal("expected force resolve by deescalate idle")
	}
}
