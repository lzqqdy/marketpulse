// Package evidence defines server-owned, time-bounded facts used by the assistant.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Status string

const (
	OK          Status = "ok"
	Partial     Status = "partial"
	Unavailable Status = "unavailable"
	Stale       Status = "stale"
)

type Evidence struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	InstrumentID   string         `json:"instrumentId,omitempty"`
	Symbol         string         `json:"symbol,omitempty"`
	AssetClass     string         `json:"assetClass,omitempty"`
	InstrumentType string         `json:"instrumentType,omitempty"`
	Source         string         `json:"source,omitempty"`
	AsOf           time.Time      `json:"asOf"`
	AsOfBasis      string         `json:"asOfBasis,omitempty"`
	FetchedAt      time.Time      `json:"fetchedAt"`
	ExpiresAt      time.Time      `json:"expiresAt"`
	Status         Status         `json:"status"`
	ChangeBasis    string         `json:"changeBasis,omitempty"`
	Currency       string         `json:"currency,omitempty"`
	Facts          map[string]any `json:"facts"`
	URL            string         `json:"url,omitempty"`
	Limitations    []string       `json:"limitations,omitempty"`
}

// Seal derives identity from the facts, not from model-supplied citation IDs.
func (e *Evidence) Seal() {
	copy := *e
	copy.ID = ""
	copy.FetchedAt = time.Time{}
	copy.ExpiresAt = time.Time{}
	b, _ := json.Marshal(copy)
	hash := sha256.Sum256(b)
	e.ID = "ev_" + hex.EncodeToString(hash[:12])
}

// Revalidate preserves historical facts but never promotes them to current data.
func (e Evidence) Revalidate(now time.Time) Evidence {
	if e.Status != Unavailable && (e.ExpiresAt.IsZero() || !now.Before(e.ExpiresAt)) {
		e.Status = Stale
	}
	return e
}
