package event

import (
	"time"
)

// Signal types (phase 1).
const (
	SignalPriceDrop       = "PRICE_DROP"
	SignalPriceSpike      = "PRICE_SPIKE"
	SignalVolumeSpike     = "VOLUME_SPIKE"
	SignalVolatilitySpike = "VOLATILITY_SPIKE"
	SignalLiquidationSpike = "LIQUIDATION_SPIKE"
)

// Event types / subtypes.
const (
	TypeCryptoMove   = "CRYPTO_MOVE"
	TypePriceAnomaly = "PRICE_ANOMALY"

	SubFlashSelloff = "FLASH_SELLOFF"
	SubVolumeMove   = "VOLUME_MOVE"
	SubDrop         = "DROP"
	SubSpike        = "SPIKE"
)

// Lifecycle statuses.
const (
	StatusDetected     = "DETECTED"
	StatusActive       = "ACTIVE"
	StatusDeescalating = "DEESCALATING"
	StatusResolved     = "RESOLVED"
)

// Severity levels.
const (
	SeverityNormal  = "NORMAL"
	SeverityLow     = "LOW"
	SeverityMedium  = "MEDIUM"
	SeverityHigh    = "HIGH"
	SeverityExtreme = "EXTREME"
)

const (
	MarketCrypto     = "CRYPTO"
	SymbolMarketWide = "MARKET"
	DirectionUp      = "UP"
	DirectionDown    = "DOWN"
	DirectionNeutral = "NEUTRAL"
)

// EventSignal is a single measurable anomaly fact.
type EventSignal struct {
	ID         string         `json:"id"`
	EventID    string         `json:"eventId,omitempty"`
	SignalType string         `json:"signalType"`
	Symbol     string         `json:"symbol"`
	Market     string         `json:"market"`
	Value      float64        `json:"value"`
	Baseline   float64        `json:"baseline"`
	Ratio      float64        `json:"ratio"`
	ChangePct  float64        `json:"changePct"`
	Window     string         `json:"window"`
	Direction  string         `json:"direction"`
	Timestamp  time.Time      `json:"timestamp"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// MarketEvent is an aggregated market incident.
type MarketEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	SubType     string         `json:"subType"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Severity    string         `json:"severity"`
	Score       float64        `json:"score"`
	PeakScore   float64        `json:"peakScore"`
	Status      string         `json:"status"`
	StartTime   time.Time      `json:"startTime"`
	EndTime     *time.Time     `json:"endTime"`
	Symbols     []string       `json:"symbols"`
	Markets     []string       `json:"markets"`
	Signals     []EventSignal  `json:"signals,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	TemplateID  string         `json:"-"` // in-memory aggregation key helper
}

// TimelineItem is one timeline node for an event.
type TimelineItem struct {
	Ts         int64          `json:"ts"`
	Kind       string         `json:"kind"` // signal | status
	Label      string         `json:"label"`
	SignalType string         `json:"signalType,omitempty"`
	Symbol     string         `json:"symbol,omitempty"`
	Status     string         `json:"status,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

// ListFilter for querying events.
type ListFilter struct {
	Market   string
	Type     string
	SubType  string
	Severity string
	Status   string
	Symbol   string
	Start    *time.Time
	End      *time.Time
	Limit    int
	Offset   int
}

// ListResult is a page of events.
type ListResult struct {
	Items      []MarketEvent `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
	Limit      int           `json:"limit"`
}

// ComponentScores holds per-signal-family scores before weighting.
type ComponentScores struct {
	Price        *float64
	Volume       *float64
	Volatility   *float64
	Liquidation  *float64
}
