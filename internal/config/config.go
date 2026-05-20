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
	Alpha   AlphaConfig  `yaml:"alpha"`
	Ingest  IngestConfig `yaml:"ingest"`
}

// AppConfig holds HTTP server settings.
type AppConfig struct {
	Addr      string `yaml:"addr"`
	Mode      string `yaml:"mode"`       // debug | release
	StaticDir string `yaml:"static_dir"` // optional: Vite dist for IP:port single-port deploy
	LogDir    string `yaml:"log_dir"`    // directory for daily level-separated logs
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

// AlphaConfig configures Binance Alpha / tokenized stocks reference quotes.
type AlphaConfig struct {
	Enabled    bool        `yaml:"enabled"`
	QuoteAsset string      `yaml:"quote_asset"`
	Indices    []AlphaItem `yaml:"indices"`
	Stocks     []AlphaItem `yaml:"stocks"`
}

// AlphaItem maps a display id/name to a Binance Alpha pair symbol.
type AlphaItem struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Symbol string `yaml:"symbol"`
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
	Interval  time.Duration `yaml:"interval"`
	IndexIDs  []string      `yaml:"index_ids"`
	Providers []string      `yaml:"providers"`
}

// DefaultEquityIndexIDs is the production watchlist (中国2 + 香港1 + 日韩2 + 美国3 + 黄金1).
var DefaultEquityIndexIDs = []string{
	"sh000001", "sz399001",
	"hsi",
	"n225", "ks11",
	"dji", "ixic", "gspc",
	"gold",
}

var DefaultAlphaIndices = []AlphaItem{
	{ID: "qqqon", Name: "QQQ", Symbol: "QQQONUSDT"},
	{ID: "spyon", Name: "SPY", Symbol: "SPYONUSDT"},
}

var DefaultAlphaStocks = []AlphaItem{
	{ID: "aaplon", Name: "AAPL", Symbol: "AAPLONUSDT"},
	{ID: "msfton", Name: "MSFT", Symbol: "MSFTONUSDT"},
	{ID: "nvdaon", Name: "NVDA", Symbol: "NVDAONUSDT"},
	{ID: "amznon", Name: "AMZN", Symbol: "AMZNONUSDT"},
	{ID: "googlon", Name: "GOOGL", Symbol: "GOOGLONUSDT"},
	{ID: "metaon", Name: "META", Symbol: "METAONUSDT"},
	{ID: "tslaon", Name: "TSLA", Symbol: "TSLAONUSDT"},
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
	if c.App.LogDir == "" {
		c.App.LogDir = "log"
	}
	if c.Ingest.Binance.WSBase == "" {
		c.Ingest.Binance.WSBase = "wss://stream.binance.com:9443/stream"
	}
	if c.Alpha.QuoteAsset == "" {
		c.Alpha.QuoteAsset = "USDT"
	}
	if len(c.Alpha.Indices) == 0 {
		c.Alpha.Indices = append([]AlphaItem(nil), DefaultAlphaIndices...)
	}
	if len(c.Alpha.Stocks) == 0 {
		c.Alpha.Stocks = append([]AlphaItem(nil), DefaultAlphaStocks...)
	}
	c.Alpha.QuoteAsset = strings.ToUpper(strings.TrimSpace(c.Alpha.QuoteAsset))
	c.Alpha.Indices = normalizeAlphaItems(c.Alpha.Indices, c.Alpha.QuoteAsset)
	c.Alpha.Stocks = normalizeAlphaItems(c.Alpha.Stocks, c.Alpha.QuoteAsset)
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
		c.Ingest.Equity.Providers = []string{"sina", "tencent", "eastmoney"}
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
		case "sina", "eastmoney", "tencent":
			normalizedProviders = append(normalizedProviders, name)
		}
	}
	if len(normalizedProviders) == 0 {
		normalizedProviders = []string{"sina", "tencent", "eastmoney"}
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

func normalizeAlphaItems(items []AlphaItem, quoteAsset string) []AlphaItem {
	out := make([]AlphaItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.ID = strings.ToLower(strings.TrimSpace(item.ID))
		item.Name = strings.TrimSpace(item.Name)
		item.Symbol = strings.ToUpper(strings.TrimSpace(item.Symbol))
		if item.Symbol == "" && item.ID != "" {
			item.Symbol = strings.ToUpper(item.ID) + quoteAsset
		}
		if item.ID == "" {
			item.ID = strings.ToLower(strings.TrimSuffix(item.Symbol, quoteAsset))
		}
		if item.Name == "" {
			item.Name = strings.ToUpper(strings.TrimSuffix(item.Symbol, quoteAsset))
		}
		if item.ID == "" || item.Symbol == "" {
			continue
		}
		if _, ok := seen[item.Symbol]; ok {
			continue
		}
		seen[item.Symbol] = struct{}{}
		out = append(out, item)
	}
	return out
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

func (c *Config) AlphaItems() []AlphaItem {
	out := make([]AlphaItem, 0, len(c.Alpha.Indices)+len(c.Alpha.Stocks))
	out = append(out, c.Alpha.Indices...)
	out = append(out, c.Alpha.Stocks...)
	return out
}

func (c *Config) AlphaBaseSymbols() []string {
	out := make([]string, 0, len(c.Alpha.Indices)+len(c.Alpha.Stocks))
	for _, item := range c.AlphaItems() {
		base := strings.TrimSuffix(strings.ToUpper(item.Symbol), c.Alpha.QuoteAsset)
		if base != "" {
			out = append(out, base)
		}
	}
	return out
}

func (c *Config) AlphaByBaseSymbol(symbol string) (AlphaItem, string, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	for _, item := range c.Alpha.Indices {
		if strings.TrimSuffix(item.Symbol, c.Alpha.QuoteAsset) == symbol {
			return item, "index", true
		}
	}
	for _, item := range c.Alpha.Stocks {
		if strings.TrimSuffix(item.Symbol, c.Alpha.QuoteAsset) == symbol {
			return item, "stock", true
		}
	}
	return AlphaItem{}, "", false
}

func (c *Config) IsAlphaBaseSymbol(symbol string) bool {
	_, _, ok := c.AlphaByBaseSymbol(symbol)
	return ok
}

func (c *Config) DayOpenSymbols() []string {
	return append([]string(nil), c.Symbols...)
}
