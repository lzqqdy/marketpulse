package server

import (
	"database/sql"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
)

// Deps bundles server dependencies.
type Deps struct {
	Config     *config.Config
	MarketData marketdata.MarketDataService
	MySQL      *sql.DB             // optional; nil when mysql.enabled=false
	Redis      *platformredis.Client // optional; nil when redis.enabled=false
}
