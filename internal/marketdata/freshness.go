package marketdata

import (
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/equity"
)

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

// IndexObservationMaxAge follows the existing ingest windows, not an exchange calendar.
// Closed markets are polled hourly; observation freshness is separate from last trade time.
func IndexObservationMaxAge(id string, now time.Time) time.Duration {
	if equity.IsMarketActive(id, now) {
		return 2 * time.Minute
	}
	return 90 * time.Minute
}
