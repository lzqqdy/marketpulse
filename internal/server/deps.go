package server

import (
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

// Deps bundles server dependencies.
type Deps struct {
	Config     *config.Config
	MarketData marketdata.MarketDataService
}
