package portfolio

import "time"

const (
	AssetTypeCrypto = "crypto"
	AssetTypeAlpha  = "alpha"

	SnapshotKindDaily     = "daily"
	SnapshotKindPrincipal = "principal"

	SourceSystem = "system"
	SourceLegacy = "legacy"
	SourceManual = "manual"
)

// Holding is a persisted user position.
type Holding struct {
	ID          int64      `json:"id,omitempty"`
	UserID      int64      `json:"-"`
	AssetType   string     `json:"assetType"`
	Symbol      string     `json:"symbol"`
	Quantity    float64    `json:"quantity"`
	TargetPrice *float64   `json:"targetPrice,omitempty"`
	CreatedAt   time.Time  `json:"-"`
	UpdatedAt   time.Time  `json:"-"`
}

// HoldingView is a holding with live valuation.
type HoldingView struct {
	AssetType  string  `json:"assetType"`
	Symbol     string  `json:"symbol"`
	Quantity   float64 `json:"quantity"`
	PriceUsdt  float64 `json:"priceUsdt"`
	ValueUsdt  float64 `json:"valueUsdt"`
	ValueCny   float64 `json:"valueCny"`
	ChangeCny  float64 `json:"changeCny"`
	Missing    bool    `json:"missing,omitempty"`
}

// Settings is user-level portfolio config.
type Settings struct {
	UserID        int64     `json:"-"`
	PrincipalCny  float64   `json:"principalCny"`
	PrincipalUsdt *float64  `json:"principalUsdt,omitempty"`
	CreatedAt     time.Time `json:"-"`
	UpdatedAt     time.Time `json:"-"`
}

// Snapshot is a daily (or principal) portfolio snapshot.
type Snapshot struct {
	ID               int64     `json:"id,omitempty"`
	UserID           int64     `json:"-"`
	Date             string    `json:"date"` // YYYY-MM-DD
	Kind             string    `json:"kind,omitempty"`
	TotalValue       float64   `json:"totalValue"`
	TotalValueCny    float64   `json:"totalValueCny"`
	DailyProfit      float64   `json:"dailyProfit"`
	DailyProfitRate  float64   `json:"dailyProfitRate"`
	TotalProfit      float64   `json:"totalProfit"`
	TotalProfitRate  float64   `json:"totalProfitRate"`
	AssetDetail      string    `json:"-"`
	Source           string    `json:"source,omitempty"`
	CreatedAt        time.Time `json:"-"`
}

// AssetDetailRow is one line inside snapshot asset_detail JSON.
type AssetDetailRow struct {
	AssetType  string  `json:"asset_type"`
	Symbol     string  `json:"symbol"`
	Quantity   float64 `json:"quantity"`
	PriceUsdt  float64 `json:"price_usdt"`
	ValueUsdt  float64 `json:"value_usdt"`
	ValueCny   float64 `json:"value_cny"`
	Raw        any     `json:"raw,omitempty"`
}

// PnLWindow is a period P&L in CNY with percentage display units (e.g. 3.11 = 3.11%).
type PnLWindow struct {
	PnlCny float64  `json:"pnlCny"`
	PnlPct *float64 `json:"pnlPct"`
}

// HoldingsResult is GET /holdings response.
type HoldingsResult struct {
	Holdings        []HoldingView `json:"holdings"`
	PrincipalCny    float64       `json:"principalCny"`
	UsdtCny         float64       `json:"usdtCny"`
	UsdtPremiumPct  float64       `json:"usdtPremiumPct"`
	RateFallback    bool          `json:"rateFallback,omitempty"`
	MissingSymbols  []string      `json:"missingSymbols,omitempty"`
}

// Overview is GET /overview response.
type Overview struct {
	TotalUsdt       float64    `json:"totalUsdt"`
	TotalCny        float64    `json:"totalCny"`
	UsdtCny         float64   `json:"usdtCny"`
	UsdtPremiumPct  float64    `json:"usdtPremiumPct"`
	RateFallback    bool       `json:"rateFallback,omitempty"`
	Today           *PnLWindow `json:"today"`
	D7              *PnLWindow `json:"d7"`
	D30             *PnLWindow `json:"d30"`
	AllTime         *PnLWindow `json:"allTime"`
	MissingSymbols  []string   `json:"missingSymbols"`
}

// PutHoldingsInput is PUT /holdings body.
type PutHoldingsInput struct {
	Holdings []HoldingInput `json:"holdings"`
}

// HoldingInput is one holding upsert row.
type HoldingInput struct {
	AssetType string  `json:"assetType"`
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
}

// PutSettingsInput is PUT /settings body.
type PutSettingsInput struct {
	PrincipalCny float64 `json:"principalCny"`
}

// ListSnapshotsQuery filters snapshot list.
type ListSnapshotsQuery struct {
	Page      int
	PageSize  int
	From      string
	To        string
	SortBy    string
	SortOrder string
}

// ListSnapshotsResult is GET /snapshots response.
type ListSnapshotsResult struct {
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
	Items    []Snapshot `json:"items"`
}

// EligibleSymbolsResult is GET /eligible-symbols response.
type EligibleSymbolsResult struct {
	Crypto []EligibleSymbol `json:"crypto"`
	Alpha  []EligibleSymbol `json:"alpha"`
}

// EligibleSymbol is a selectable mark.
type EligibleSymbol struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name,omitempty"`
	Type   string `json:"assetType"`
}

// ReportSeriesPoint is one day on the report charts.
type ReportSeriesPoint struct {
	Date             string  `json:"date"`
	TotalValue       float64 `json:"totalValue"`
	TotalValueCny    float64 `json:"totalValueCny"`
	DailyProfit      float64 `json:"dailyProfit"`
	DailyProfitRate  float64 `json:"dailyProfitRate"`
	TotalProfit      float64 `json:"totalProfit"`
	TotalProfitRate  float64 `json:"totalProfitRate"`
}

// ReportSeriesSummary is the period strip above charts.
type ReportSeriesSummary struct {
	StartCny float64  `json:"startCny"`
	EndCny   float64  `json:"endCny"`
	PnlCny   float64  `json:"pnlCny"`
	PnlPct   *float64 `json:"pnlPct"`
}

// ReportSeriesResult is GET /reports/series.
type ReportSeriesResult struct {
	Range   string               `json:"range"`
	From    string               `json:"from"`
	To      string               `json:"to"`
	Summary ReportSeriesSummary  `json:"summary"`
	Points  []ReportSeriesPoint  `json:"points"`
}

// AllocationItem is one slice of the allocation donut.
type AllocationItem struct {
	AssetType string  `json:"assetType"`
	Symbol    string  `json:"symbol"`
	ValueCny  float64 `json:"valueCny"`
	ValueUsdt float64 `json:"valueUsdt"`
	WeightPct float64 `json:"weightPct"`
}

// AllocationResult is GET /reports/allocation.
type AllocationResult struct {
	TotalCny       float64          `json:"totalCny"`
	TotalUsdt      float64          `json:"totalUsdt"`
	Items          []AllocationItem `json:"items"`
	MissingSymbols []string         `json:"missingSymbols"`
	RateFallback   bool             `json:"rateFallback,omitempty"`
}
