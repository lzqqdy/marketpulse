package event

import "time"

// StillAnomalous reports whether current strength still exceeds resolve floor.
func StillAnomalous(score float64) bool {
	return score >= 30
}

// NextStatus advances lifecycle given current status and whether anomaly persists / is weakening.
//
//	DETECTED → ACTIVE (always on next evaluation while anomalous)
//	ACTIVE → DEESCALATING when score drops meaningfully below peak but still anomalous
//	DEESCALATING → ACTIVE if score climbs back near peak; → RESOLVED if not anomalous
//	RESOLVED stays RESOLVED
func NextStatus(current string, score, peakScore float64, anomalous bool) string {
	switch current {
	case StatusResolved:
		return StatusResolved
	case StatusDetected:
		if anomalous {
			return StatusActive
		}
		return StatusResolved
	case StatusActive:
		if !anomalous {
			return StatusResolved
		}
		if peakScore > 0 && score < peakScore*0.75 {
			return StatusDeescalating
		}
		return StatusActive
	case StatusDeescalating:
		if !anomalous {
			return StatusResolved
		}
		if peakScore > 0 && score >= peakScore*0.9 {
			return StatusActive
		}
		return StatusDeescalating
	default:
		if anomalous {
			return StatusActive
		}
		return StatusResolved
	}
}

// ApplyLifecycle mutates event status/endTime/peakScore based on new score.
func ApplyLifecycle(ev *MarketEvent, score float64, now time.Time) {
	if score > ev.PeakScore {
		ev.PeakScore = score
	}
	ev.Score = score
	ev.Severity = SeverityFromScore(score)
	anomalous := StillAnomalous(score)
	next := NextStatus(ev.Status, score, ev.PeakScore, anomalous)
	if next == StatusResolved && ev.Status != StatusResolved {
		t := now.UTC()
		ev.EndTime = &t
	}
	if next != StatusResolved {
		ev.EndTime = nil
	}
	ev.Status = next
	ev.UpdatedAt = now.UTC()
}

// ForceResolve marks an open event resolved (timeout / dedupe cleanup).
func ForceResolve(ev *MarketEvent, now time.Time) {
	if ev.Status == StatusResolved {
		return
	}
	t := now.UTC()
	ev.Status = StatusResolved
	ev.EndTime = &t
	ev.UpdatedAt = t
	if ev.Score >= 30 {
		ev.Score = 25
		ev.Severity = SeverityFromScore(ev.Score)
	}
}

// ShouldForceResolve reports age/idle based hard close.
func ShouldForceResolve(ev *MarketEvent, now time.Time, maxOpen, maxDeesc time.Duration) bool {
	if ev.Status == StatusResolved {
		return false
	}
	now = now.UTC()
	start := ev.StartTime.UTC()
	updated := ev.UpdatedAt.UTC()
	if maxOpen > 0 && !start.IsZero() {
		age := now.Sub(start)
		if age < 0 {
			age = -age
		}
		if age >= maxOpen {
			return true
		}
	}
	if ev.Status == StatusDeescalating && maxDeesc > 0 && !updated.IsZero() {
		idle := now.Sub(updated)
		if idle < 0 {
			idle = -idle
		}
		if idle >= maxDeesc {
			return true
		}
	}
	return false
}
