package store

import "time"

// Quote is a spot price row (RFC-002).
type Quote struct {
	Symbol       string    `json:"symbol"`
	PriceUsdt    float64   `json:"priceUsdt"`
	PriceCny     float64   `json:"priceCny"`
	ChangeDayPct float64   `json:"changeDayPct"`
	Change24hPct float64   `json:"change24hPct"`
	Rank         int       `json:"rank,omitempty"`
	IconURL      string    `json:"iconUrl,omitempty"`
	MarketCapUsd float64   `json:"marketCapUsd,omitempty"`
	Volume24hUsd float64   `json:"volume24hUsd,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Rates holds fiat conversion rates.
type Rates struct {
	USDTCNY   float64   `json:"usdtCny"`
	USDCNY    float64   `json:"usdCny"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// IndexQuote is a stock / gold index row.
type IndexQuote struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	ChangePct float64   `json:"changePct"`
	Source    string    `json:"source,omitempty"`
	Stale     bool      `json:"stale"`
	FetchedAt time.Time `json:"fetchedAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AlphaQuote is a Binance Alpha / tokenized stocks reference quote.
type AlphaQuote struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Symbol       string    `json:"symbol"`
	Price        float64   `json:"price"`
	Change24hPct float64   `json:"change24hPct"`
	ChangeDayPct float64   `json:"changeDayPct"`
	Volume       float64   `json:"volume"`
	MarkPrice    float64   `json:"markPrice,omitempty"`
	IndexPrice   float64   `json:"indexPrice,omitempty"`
	FundingRate  float64   `json:"fundingRate,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Source       string    `json:"source"`
	Category     string    `json:"category"` // index | stock
}

// AlphaSnapshot groups Binance Alpha reference rows.
type AlphaSnapshot struct {
	Indices   []AlphaQuote `json:"indices"`
	Stocks    []AlphaQuote `json:"stocks"`
	UpdatedAt time.Time    `json:"updatedAt,omitempty"`
	Source    string       `json:"source,omitempty"`
}

// MacroSnapshot holds slow macro indicators.
type MacroSnapshot struct {
	TotalMarketCapUsd               float64        `json:"totalMarketCapUsd"`
	TotalVolume24hUsd               float64        `json:"totalVolume24hUsd"`
	TotalMarketCapChange24hPct      float64        `json:"totalMarketCapChange24hPct"`
	FearGreed                       FearGreed      `json:"fearGreed"`
	BTCDominancePct                 float64        `json:"btcDominancePct"`
	ETHDominancePct                 float64        `json:"ethDominancePct"`
	StablecoinMarketCapUsd          float64        `json:"stablecoinMarketCapUsd"`
	StablecoinMarketCapChange24hPct float64        `json:"stablecoinMarketCapChange24hPct"`
	LongShort                       LongShortRatio `json:"longShort"`
	TopLongShort                    LongShortRatio `json:"topLongShort"`
	Funding                         FundingRate    `json:"funding"`
	OpenInterest                    OpenInterest   `json:"openInterest"`
	TakerBuySell                    TakerBuySell   `json:"takerBuySell"`
	Liquidations                    Liquidations   `json:"liquidations"`
}

// FearGreed is the alternative.me style sentiment bucket.
type FearGreed struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// LongShortRatio is a futures sentiment snapshot.
type LongShortRatio struct {
	Symbol          string    `json:"symbol"`
	Ratio           float64   `json:"ratio"`
	LongAccountPct  float64   `json:"longAccountPct"`
	ShortAccountPct float64   `json:"shortAccountPct"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// FundingRate is the latest futures funding snapshot.
type FundingRate struct {
	Symbol          string    `json:"symbol"`
	Rate            float64   `json:"rate"`
	MarkPrice       float64   `json:"markPrice,omitempty"`
	IndexPrice      float64   `json:"indexPrice,omitempty"`
	PremiumPct      float64   `json:"premiumPct,omitempty"`
	NextFundingTime time.Time `json:"nextFundingTime"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// OpenInterest is a futures open interest snapshot.
type OpenInterest struct {
	Symbol    string    `json:"symbol"`
	ValueUsd  float64   `json:"valueUsd"`
	ChangePct float64   `json:"changePct"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TakerBuySell is taker-side futures volume.
type TakerBuySell struct {
	Symbol    string    `json:"symbol"`
	Ratio     float64   `json:"ratio"`
	BuyVol    float64   `json:"buyVol"`
	SellVol   float64   `json:"sellVol"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Liquidations is a rolling forced-liquidation aggregate.
type Liquidations struct {
	Window    string    `json:"window"`
	LongUsd   float64   `json:"longUsd"`
	ShortUsd  float64   `json:"shortUsd"`
	TotalUsd  float64   `json:"totalUsd"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Snapshot is the full market state served to clients (RFC-002).
type Snapshot struct {
	Version uint64        `json:"version"`
	Ts      int64         `json:"ts"`
	Quotes  []Quote       `json:"quotes"`
	Rates   Rates         `json:"rates"`
	Indices []IndexQuote  `json:"indices"`
	Alpha   AlphaSnapshot `json:"alpha"`
	Macro   MacroSnapshot `json:"macro"`
}
