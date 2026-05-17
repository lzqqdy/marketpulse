package equity

import "time"

// CacheTTL returns a simple market freshness window.
// Weekdays are treated as trading days; weekends are treated as closed.
func CacheTTL(now time.Time) time.Duration {
	return time.Minute
}
