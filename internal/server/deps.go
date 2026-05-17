package server

import (
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/hub"
	"github.com/lzqqdy/marketpulse/internal/ingest"
	"github.com/lzqqdy/marketpulse/internal/store"
)

// Deps bundles server dependencies.
type Deps struct {
	Config    *config.Config
	Store     *store.MarketStore
	StreamHub *hub.StreamHub
	KlineHub  *hub.KlineHub
	Ingest    *ingest.Service
}
