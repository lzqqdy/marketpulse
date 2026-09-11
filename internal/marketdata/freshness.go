package marketdata

import "time"

const (
	SpotQuoteMaxAge      = 2 * time.Minute
	ReferenceQuoteMaxAge = 5 * time.Minute
	ConversionRateMaxAge = 5 * time.Minute
)

// IsStale treats unknown/future timestamps as unverified, not fresh.
// Reference feeds are polled even when the underlying stock market is closed.
func IsStale(updatedAt, now time.Time, maxAge time.Duration) bool {
	return updatedAt.IsZero() || updatedAt.After(now.Add(time.Minute)) || now.Sub(updatedAt) > maxAge
}
