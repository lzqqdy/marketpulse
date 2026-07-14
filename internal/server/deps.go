package server

import (
	"database/sql"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	platformredis "github.com/lzqqdy/marketpulse/internal/platform/redis"
	"github.com/lzqqdy/marketpulse/internal/platform/upload"
	"github.com/lzqqdy/marketpulse/internal/users"
)

// Deps bundles server dependencies.
type Deps struct {
	Config     *config.Config
	MarketData marketdata.MarketDataService
	Users      users.Service         // optional; nil / disabled without mysql+redis
	Upload     *upload.Store         // local file store for avatars
	MySQL      *sql.DB               // optional; nil when mysql.enabled=false
	Redis      *platformredis.Client // optional; nil when redis.enabled=false
}
