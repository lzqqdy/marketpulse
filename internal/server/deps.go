package server

import (
	"database/sql"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/platform/upload"
	"github.com/lzqqdy/marketpulse/internal/alerts"
	"github.com/lzqqdy/marketpulse/internal/users"
)

// Deps bundles server dependencies.
type Deps struct {
	Config      *config.Config
	MarketData  marketdata.MarketDataService
	Users       users.Service
	Alerts      alerts.Service
	AlertStream *alerts.StreamServer
	Upload      *upload.Store
	MySQL       *sql.DB
	Redis       *platformredis.Client
}
