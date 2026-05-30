package equity

import "time"

const (
	ActiveTTL   = time.Minute
	InactiveTTL = time.Hour
)

var marketLocation = time.FixedZone("CST", 8*60*60)

type MarketWindow struct {
	Start string
	End   string
}

var ActiveWindows = map[string]MarketWindow{
	"sh000001": {"09:15", "15:15"},
	"sz399001": {"09:15", "15:15"},
	"sz399006": {"09:15", "15:15"},
	"sh000300": {"09:15", "15:15"},
	"sh000688": {"09:15", "15:15"},
	"hsi":      {"09:15", "16:15"},
	"n225":     {"07:45", "14:45"},
	"ks11":     {"07:45", "14:45"},
	"dji":      {"21:15", "05:15"},
	"ixic":     {"21:15", "05:15"},
	"gspc":     {"21:15", "05:15"},
	"gold":     {"05:45", "05:15"},
	"silver":   {"05:45", "05:15"},
	"crude":    {"05:45", "05:15"},
}

// CacheTTL keeps active markets fresh at 60s and lets closed markets cool down.
func CacheTTL(def IndexDef, now time.Time) time.Duration {
	if IsMarketActive(def.ID, now) {
		return ActiveTTL
	}
	return InactiveTTL
}

func AnyMarketActive(defs []IndexDef, now time.Time) bool {
	for _, def := range defs {
		if IsMarketActive(def.ID, now) {
			return true
		}
	}
	return false
}

func NextPollInterval(defs []IndexDef, now time.Time) time.Duration {
	if AnyMarketActive(defs, now) {
		return ActiveTTL
	}
	next := InactiveTTL
	for _, def := range defs {
		if d, ok := nextActiveStart(def.ID, now); ok && d > 0 && d < next {
			next = d
		}
	}
	if next < ActiveTTL {
		return ActiveTTL
	}
	return next
}

func IsMarketActive(id string, now time.Time) bool {
	w, ok := ActiveWindows[id]
	if !ok {
		return false
	}
	local := now.In(marketLocation)
	start, ok := parseClock(w.Start)
	if !ok {
		return false
	}
	end, ok := parseClock(w.End)
	if !ok {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	if start <= end {
		return isWeekday(local) && minute >= start && minute <= end
	}
	if minute >= start {
		return isWeekday(local)
	}
	if minute <= end {
		return isWeekday(local.AddDate(0, 0, -1))
	}
	return false
}

func nextActiveStart(id string, now time.Time) (time.Duration, bool) {
	w, ok := ActiveWindows[id]
	if !ok {
		return 0, false
	}
	start, ok := parseClock(w.Start)
	if !ok {
		return 0, false
	}
	local := now.In(marketLocation)
	base := time.Date(local.Year(), local.Month(), local.Day(), start/60, start%60, 0, 0, marketLocation)
	for i := 0; i < 8; i++ {
		candidate := base.AddDate(0, 0, i)
		if candidate.After(local) && isWeekday(candidate) {
			return candidate.Sub(local), true
		}
	}
	return 0, false
}

func parseClock(s string) (int, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

func isWeekday(t time.Time) bool {
	day := t.Weekday()
	return day >= time.Monday && day <= time.Friday
}
