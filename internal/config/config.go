// Package config loads application settings from YAML and environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration for marketd.
type Config struct {
	App     AppConfig    `yaml:"app"`
	CORS    CORSConfig   `yaml:"cors"`
	Symbols []string     `yaml:"symbols"`
	Ingest  IngestConfig `yaml:"ingest"`
}

// AppConfig holds HTTP server settings.
type AppConfig struct {
	Addr      string `yaml:"addr"`
	Mode      string `yaml:"mode"`       // debug | release
	StaticDir string `yaml:"static_dir"` // optional: Vite dist for IP:port single-port deploy
}

// CORSConfig lists allowed browser origins (dev).
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// IngestConfig holds poller / websocket intervals (used from Phase B onward).
type IngestConfig struct {
	Binance BinanceConfig `yaml:"binance"`
	OTC     OTCConfig     `yaml:"otc"`
	Forex   ForexConfig   `yaml:"forex"`
	Equity  EquityConfig  `yaml:"equity"`
	Macro   MacroConfig   `yaml:"macro"`
}

// BinanceConfig configures the Binance websocket feed.
type BinanceConfig struct {
	WSBase string `yaml:"ws_base"`
}

// OTCConfig configures USDT/CNY polling.
type OTCConfig struct {
	USDTCNYInterval time.Duration `yaml:"usdt_cny_interval"`
}

// ForexConfig configures fiat FX polling.
type ForexConfig struct {
	USDCNYInterval time.Duration `yaml:"usd_cny_interval"`
}

// EquityConfig configures stock index polling.
type EquityConfig struct {
	Interval         time.Duration `yaml:"interval"`
	IndexIDs         []string      `yaml:"index_ids"`
	Providers        []string      `yaml:"providers"`
	FinnhubAPIKey    string        `yaml:"finnhub_api_key"`
	TwelveDataAPIKey string        `yaml:"twelvedata_api_key"`
}

// DefaultEquityIndexIDs is the production watchlist (中国2 + 日韩2 + 美国3 + 黄金1).
var DefaultEquityIndexIDs = []string{
	"sh000001", "sz399001",
	"n225", "ks11",
	"dji", "ixic", "gspc",
	"gold",
}

// MacroConfig configures macro indicator polling.
type MacroConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// Load reads YAML from path and applies environment overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()
	cfg.applyEnv()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.App.Addr == "" {
		c.App.Addr = ":8080"
	}
	if c.App.Mode == "" {
		c.App.Mode = "debug"
	}
	if c.Ingest.Binance.WSBase == "" {
		c.Ingest.Binance.WSBase = "wss://stream.binance.com:9443/stream"
	}
	if c.Ingest.OTC.USDTCNYInterval == 0 {
		c.Ingest.OTC.USDTCNYInterval = 30 * time.Second
	}
	if c.Ingest.Forex.USDCNYInterval == 0 {
		c.Ingest.Forex.USDCNYInterval = time.Hour
	}
	if c.Ingest.Equity.Interval == 0 {
		c.Ingest.Equity.Interval = time.Minute
	}
	if len(c.Ingest.Equity.IndexIDs) == 0 {
		c.Ingest.Equity.IndexIDs = append([]string(nil), DefaultEquityIndexIDs...)
	}
	if len(c.Ingest.Equity.Providers) == 0 {
		c.Ingest.Equity.Providers = []string{"yahoo", "twelvedata", "stooq"}
	}
	normalizedIDs := make([]string, 0, len(c.Ingest.Equity.IndexIDs))
	for _, id := range c.Ingest.Equity.IndexIDs {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" {
			normalizedIDs = append(normalizedIDs, id)
		}
	}
	c.Ingest.Equity.IndexIDs = normalizedIDs
	normalizedProviders := make([]string, 0, len(c.Ingest.Equity.Providers))
	for _, name := range c.Ingest.Equity.Providers {
		name = strings.ToLower(strings.TrimSpace(name))
		switch name {
		case "yahoo", "finnhub", "twelvedata", "stooq":
			normalizedProviders = append(normalizedProviders, name)
		}
	}
	if len(normalizedProviders) == 0 {
		normalizedProviders = []string{"yahoo", "twelvedata", "stooq"}
	}
	c.Ingest.Equity.Providers = normalizedProviders
	if c.Ingest.Macro.Interval == 0 {
		c.Ingest.Macro.Interval = 5 * time.Minute
	}

	normalized := make([]string, 0, len(c.Symbols))
	for _, s := range c.Symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" {
			normalized = append(normalized, s)
		}
	}
	c.Symbols = normalized
}

func (c *Config) applyEnv() {
	if v := os.Getenv("MARKETPULSE_APP_ADDR"); v != "" {
		c.App.Addr = v
	}
	if v := os.Getenv("MARKETPULSE_APP_MODE"); v != "" {
		c.App.Mode = v
	}
	if v := os.Getenv("MARKETPULSE_BINANCE_WS_BASE"); v != "" {
		c.Ingest.Binance.WSBase = v
	}
	if v := os.Getenv("MARKETPULSE_FINNHUB_API_KEY"); v != "" {
		c.Ingest.Equity.FinnhubAPIKey = v
	}
	if v := os.Getenv("MARKETPULSE_TWELVEDATA_API_KEY"); v != "" {
		c.Ingest.Equity.TwelveDataAPIKey = v
	}
}

func (c *Config) validate() error {
	if len(c.Symbols) == 0 {
		return fmt.Errorf("config: symbols must not be empty")
	}
	switch c.App.Mode {
	case "debug", "release":
	default:
		return fmt.Errorf("config: app.mode must be debug or release, got %q", c.App.Mode)
	}
	return nil
}

// BinanceStreamURL builds a combined stream path query for configured symbols.
func (c *Config) BinanceStreamURL() string {
	parts := make([]string, 0, len(c.Symbols))
	for _, sym := range c.Symbols {
		parts = append(parts, strings.ToLower(sym)+"usdt@miniTicker")
	}
	return c.Ingest.Binance.WSBase + "?streams=" + strings.Join(parts, "/")
}
