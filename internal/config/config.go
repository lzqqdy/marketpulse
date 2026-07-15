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
	MySQL   MySQLConfig  `yaml:"mysql"`
	Redis   RedisConfig  `yaml:"redis"`
	Users   UsersConfig   `yaml:"users"`
	Alerts  AlertsConfig  `yaml:"alerts"`
	SMTP    SMTPConfig    `yaml:"smtp"`
	Upload  UploadConfig  `yaml:"upload"`
	Symbols []string     `yaml:"symbols"`
	Alpha   AlphaConfig  `yaml:"alpha"`
	Ingest  IngestConfig `yaml:"ingest"`

	// UsersSkipReason is set when users.enabled was requested but deps are missing.
	UsersSkipReason string `yaml:"-"`
	// AlertsSkipReason is set when alerts.enabled was requested but deps are missing.
	AlertsSkipReason string `yaml:"-"`
}

// AlertsConfig configures the optional alerts module (requires mysql + redis + users).
type AlertsConfig struct {
	Enabled         bool   `yaml:"enabled"`
	AutoMigrate     *bool  `yaml:"auto_migrate"`
	DailyTimezone   string `yaml:"daily_timezone"`
	LoopIntervalMin int    `yaml:"loop_interval_min"`
	LoopIntervalMax int    `yaml:"loop_interval_max"`
	InboxMaxLen     int    `yaml:"inbox_max_len"`
}

// IsAutoMigrate reports whether alerts schema migrations should run on startup.
func (c AlertsConfig) IsAutoMigrate() bool {
	if c.AutoMigrate == nil {
		return true
	}
	return *c.AutoMigrate
}

// SMTPConfig configures outbound email for alert delivery.
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

// UploadConfig controls local file storage for avatars and other user assets.
type UploadConfig struct {
	Dir            string `yaml:"dir"`              // filesystem root, relative to process cwd
	PublicPath     string `yaml:"public_path"`      // URL prefix, e.g. /uploads
	MaxAvatarBytes int64  `yaml:"max_avatar_bytes"` // default 2 MiB
}

// UsersConfig configures the optional users module (requires mysql + redis).
type UsersConfig struct {
	Enabled     bool             `yaml:"enabled"`
	AutoMigrate *bool            `yaml:"auto_migrate"` // nil => true when enabled
	SessionTTL  time.Duration    `yaml:"session_ttl"`
	Seed        UsersSeed        `yaml:"seed"`
	Security    UsersSecurityCfg `yaml:"security"`
}

// UsersSecurityCfg hardens login against brute-force / scraping.
type UsersSecurityCfg struct {
	// MaxAttemptsPerIP is the max login attempts per IP in Window (default 30).
	MaxAttemptsPerIP int `yaml:"max_attempts_per_ip"`
	// MaxAttemptsPerPhone is the max login attempts per phone in Window (default 15).
	MaxAttemptsPerPhone int `yaml:"max_attempts_per_phone"`
	// Window is the sliding counter window for IP/phone attempt caps (default 15m).
	Window time.Duration `yaml:"window"`
	// LockoutFailures locks a phone after N consecutive failed logins (default 5).
	LockoutFailures int `yaml:"lockout_failures"`
	// LockoutTTL is how long a phone stays locked after too many failures (default 15m).
	LockoutTTL time.Duration `yaml:"lockout_ttl"`
}

// IsAutoMigrate reports whether schema migrations should run on startup.
func (c UsersConfig) IsAutoMigrate() bool {
	if c.AutoMigrate == nil {
		return true
	}
	return *c.AutoMigrate
}

func (c *UsersConfig) applySecurityDefaults() {
	if c.Security.MaxAttemptsPerIP <= 0 {
		c.Security.MaxAttemptsPerIP = 30
	}
	if c.Security.MaxAttemptsPerPhone <= 0 {
		c.Security.MaxAttemptsPerPhone = 15
	}
	if c.Security.Window <= 0 {
		c.Security.Window = 15 * time.Minute
	}
	if c.Security.LockoutFailures <= 0 {
		c.Security.LockoutFailures = 5
	}
	if c.Security.LockoutTTL <= 0 {
		c.Security.LockoutTTL = 15 * time.Minute
	}
}

// UsersSeed creates an initial account when missing (no public registration).
type UsersSeed struct {
	Phone       string `yaml:"phone"`
	Password    string `yaml:"password"`
	DisplayName string `yaml:"display_name"`
}

// MySQLConfig holds relational database settings for users/alerts/portfolio modules.
// Disabled by default so existing market-only deployments stay unchanged.
type MySQLConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	Params          string        `yaml:"params"` // query string without leading '?'
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// DSN builds a go-sql-driver/mysql DSN.
func (c MySQLConfig) DSN() string {
	user := c.User
	if user == "" {
		user = "root"
	}
	host := c.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port <= 0 {
		port = 3306
	}
	params := c.Params
	if params == "" {
		params = "parseTime=true&loc=Local&charset=utf8mb4"
	}
	auth := user
	if c.Password != "" {
		auth = user + ":" + c.Password
	}
	return fmt.Sprintf("%s@tcp(%s:%d)/%s?%s", auth, host, port, c.Database, params)
}

// RedisConfig holds cache/queue settings for sessions, alerts, and jobs.
// Disabled by default so existing market-only deployments stay unchanged.
type RedisConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
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
	Baidu   BaiduConfig   `yaml:"baidu"`
	OTC     OTCConfig     `yaml:"otc"`
	Forex   ForexConfig     `yaml:"forex"`
	Equity  EquityConfig  `yaml:"equity"`
	Macro   MacroConfig   `yaml:"macro"`
}

// BinanceConfig configures the Binance websocket feed.
type BinanceConfig struct {
	WSBase string `yaml:"ws_base"`
}

// AlphaConfig configures US stock reference quotes (Bitget primary, Binance Alpha fallback).
type AlphaConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Provider        string        `yaml:"provider"`
	ProductType     string        `yaml:"product_type"`
	QuoteAsset      string        `yaml:"quote_asset"`
	PollInterval    time.Duration `yaml:"poll_interval"`
	ResolveInterval time.Duration `yaml:"resolve_interval"`
	Indices         []AlphaItem   `yaml:"indices"`
	Stocks          []AlphaItem   `yaml:"stocks"`
}

// AlphaItem maps a display id/name to an upstream pair symbol (e.g. AAPLUSDT).
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

// BaiduConfig configures Baidu Finance index ingest.
type BaiduConfig struct {
	Enabled          *bool         `yaml:"enabled"`
	BaseURL          string        `yaml:"base_url"`
	WSURL            string        `yaml:"ws_url"`
	WSEnabled        *bool         `yaml:"ws_enabled"`
	WSReconnectMax   int           `yaml:"ws_reconnect_max"`
	WSReconnectDelay time.Duration `yaml:"ws_reconnect_delay"`
	WSPatchInterval  time.Duration `yaml:"ws_patch_interval"`
}

func (c *BaiduConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

func (c *BaiduConfig) IsWSEnabled() bool {
	if c.WSEnabled == nil {
		return true
	}
	return *c.WSEnabled
}

// DefaultEquityIndexIDs is the production watchlist (中国5 + 香港1 + 日韩2 + 美国3 + 商品3).
var DefaultEquityIndexIDs = []string{
	"sh000001", "sz399001", "sz399006", "sh000300", "sh000688",
	"hsi",
	"n225", "ks11",
	"dji", "ixic", "gspc",
	"gold", "silver", "crude",
}

var DefaultAlphaIndices = []AlphaItem{
	{ID: "qqq", Name: "纳指ETF", Symbol: "QQQUSDT"},
	{ID: "spy", Name: "标普ETF", Symbol: "SPYUSDT"},
}

var DefaultAlphaStocks = []AlphaItem{
	{ID: "aapl", Name: "苹果", Symbol: "AAPLUSDT"},
	{ID: "msft", Name: "微软", Symbol: "MSFTUSDT"},
	{ID: "nvda", Name: "英伟达", Symbol: "NVDAUSDT"},
	{ID: "amzn", Name: "亚马逊", Symbol: "AMZNUSDT"},
	{ID: "googl", Name: "谷歌", Symbol: "GOOGLUSDT"},
	{ID: "meta", Name: "Meta", Symbol: "METAUSDT"},
	{ID: "tsla", Name: "特斯拉", Symbol: "TSLAUSDT"},
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
	cfg.applyUsersGuard()
	cfg.applyAlertsGuard()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// applyUsersGuard soft-disables users when mysql/redis are off so market-only boot succeeds.
func (c *Config) applyUsersGuard() {
	c.UsersSkipReason = ""
	if !c.Users.Enabled {
		return
	}
	switch {
	case !c.MySQL.Enabled && !c.Redis.Enabled:
		c.Users.Enabled = false
		c.UsersSkipReason = "mysql and redis are disabled"
	case !c.MySQL.Enabled:
		c.Users.Enabled = false
		c.UsersSkipReason = "mysql is disabled"
	case !c.Redis.Enabled:
		c.Users.Enabled = false
		c.UsersSkipReason = "redis is disabled"
	}
}

// applyAlertsGuard soft-disables alerts when mysql/redis/users are off.
func (c *Config) applyAlertsGuard() {
	c.AlertsSkipReason = ""
	if !c.Alerts.Enabled {
		return
	}
	switch {
	case !c.MySQL.Enabled && !c.Redis.Enabled && !c.Users.Enabled:
		c.Alerts.Enabled = false
		c.AlertsSkipReason = "mysql, redis and users are disabled"
	case !c.MySQL.Enabled:
		c.Alerts.Enabled = false
		c.AlertsSkipReason = "mysql is disabled"
	case !c.Redis.Enabled:
		c.Alerts.Enabled = false
		c.AlertsSkipReason = "redis is disabled"
	case !c.Users.Enabled:
		c.Alerts.Enabled = false
		c.AlertsSkipReason = "users module is disabled"
	}
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
	if c.Upload.Dir == "" {
		c.Upload.Dir = "data/uploads"
	}
	if c.Upload.PublicPath == "" {
		c.Upload.PublicPath = "/uploads"
	}
	if c.Upload.MaxAvatarBytes <= 0 {
		c.Upload.MaxAvatarBytes = 10 << 20 // 10 MiB (client also compresses before upload)
	}
	if c.Ingest.Binance.WSBase == "" {
		c.Ingest.Binance.WSBase = "wss://stream.binance.com:9443/stream"
	}
	if c.Ingest.Baidu.BaseURL == "" {
		c.Ingest.Baidu.BaseURL = "https://finance.pae.baidu.com"
	}
	if c.Ingest.Baidu.WSURL == "" {
		c.Ingest.Baidu.WSURL = "wss://finance-ws.pae.baidu.com"
	}
	if c.Ingest.Baidu.WSReconnectMax == 0 {
		c.Ingest.Baidu.WSReconnectMax = 5
	}
	if c.Ingest.Baidu.WSReconnectDelay == 0 {
		c.Ingest.Baidu.WSReconnectDelay = 3 * time.Second
	}
	if c.Ingest.Baidu.WSPatchInterval == 0 {
		c.Ingest.Baidu.WSPatchInterval = 60 * time.Second
	}
	if c.Alpha.QuoteAsset == "" {
		c.Alpha.QuoteAsset = "USDT"
	}
	if c.Alpha.Provider == "" {
		c.Alpha.Provider = "bitget"
	}
	if c.Alpha.ProductType == "" {
		c.Alpha.ProductType = "USDT-FUTURES"
	}
	if c.Alpha.PollInterval == 0 {
		c.Alpha.PollInterval = 30 * time.Second
	}
	if c.Alpha.ResolveInterval == 0 {
		c.Alpha.ResolveInterval = 10 * time.Minute
	}
	if len(c.Alpha.Indices) == 0 {
		c.Alpha.Indices = append([]AlphaItem(nil), DefaultAlphaIndices...)
	}
	if len(c.Alpha.Stocks) == 0 {
		c.Alpha.Stocks = append([]AlphaItem(nil), DefaultAlphaStocks...)
	}
	c.Alpha.QuoteAsset = strings.ToUpper(strings.TrimSpace(c.Alpha.QuoteAsset))
	c.Alpha.Provider = strings.ToLower(strings.TrimSpace(c.Alpha.Provider))
	c.Alpha.ProductType = strings.ToUpper(strings.TrimSpace(c.Alpha.ProductType))
	c.Alpha.Indices = normalizeAlphaItems(c.Alpha.Indices, c.Alpha.QuoteAsset, c.Alpha.Provider)
	c.Alpha.Stocks = normalizeAlphaItems(c.Alpha.Stocks, c.Alpha.QuoteAsset, c.Alpha.Provider)
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
		if c.Ingest.Baidu.IsEnabled() {
			c.Ingest.Equity.Providers = []string{"baidu", "tencent", "eastmoney"}
		} else {
			c.Ingest.Equity.Providers = []string{"tencent", "eastmoney"}
		}
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
		case "baidu", "eastmoney", "tencent":
			normalizedProviders = append(normalizedProviders, name)
		}
	}
	if len(normalizedProviders) == 0 {
		if c.Ingest.Baidu.IsEnabled() {
			normalizedProviders = []string{"baidu", "tencent", "eastmoney"}
		} else {
			normalizedProviders = []string{"tencent", "eastmoney"}
		}
	}
	if c.Ingest.Baidu.IsEnabled() {
		hasBaidu := false
		for _, name := range normalizedProviders {
			if name == "baidu" {
				hasBaidu = true
				break
			}
		}
		if !hasBaidu {
			normalizedProviders = append([]string{"baidu"}, normalizedProviders...)
		}
	}
	c.Ingest.Equity.Providers = normalizedProviders
	if c.Ingest.Macro.Interval == 0 {
		c.Ingest.Macro.Interval = 5 * time.Minute
	}

	if c.MySQL.Host == "" {
		c.MySQL.Host = "127.0.0.1"
	}
	if c.MySQL.Port <= 0 {
		c.MySQL.Port = 3306
	}
	if c.MySQL.User == "" {
		c.MySQL.User = "root"
	}
	if c.MySQL.Database == "" {
		c.MySQL.Database = "marketpulse"
	}
	if c.MySQL.Params == "" {
		c.MySQL.Params = "parseTime=true&loc=Local&charset=utf8mb4"
	}
	if c.MySQL.MaxOpenConns <= 0 {
		c.MySQL.MaxOpenConns = 20
	}
	if c.MySQL.MaxIdleConns <= 0 {
		c.MySQL.MaxIdleConns = 5
	}
	if c.MySQL.ConnMaxLifetime <= 0 {
		c.MySQL.ConnMaxLifetime = time.Hour
	}
	if c.MySQL.ConnMaxIdleTime <= 0 {
		c.MySQL.ConnMaxIdleTime = 10 * time.Minute
	}

	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Redis.PoolSize <= 0 {
		c.Redis.PoolSize = 10
	}
	if c.Redis.DialTimeout <= 0 {
		c.Redis.DialTimeout = 5 * time.Second
	}
	if c.Redis.ReadTimeout <= 0 {
		c.Redis.ReadTimeout = 3 * time.Second
	}
	if c.Redis.WriteTimeout <= 0 {
		c.Redis.WriteTimeout = 3 * time.Second
	}

	if c.Users.SessionTTL <= 0 {
		c.Users.SessionTTL = 7 * 24 * time.Hour
	}
	c.Users.applySecurityDefaults()

	if c.Alerts.DailyTimezone == "" {
		c.Alerts.DailyTimezone = "Asia/Shanghai"
	}
	if c.Alerts.LoopIntervalMin <= 0 {
		c.Alerts.LoopIntervalMin = 1
	}
	if c.Alerts.LoopIntervalMax <= 0 {
		c.Alerts.LoopIntervalMax = 1440
	}
	if c.Alerts.InboxMaxLen <= 0 {
		c.Alerts.InboxMaxLen = 100
	}
	if c.SMTP.Port <= 0 {
		c.SMTP.Port = 465
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

func normalizeAlphaItems(items []AlphaItem, quoteAsset string, provider string) []AlphaItem {
	out := make([]AlphaItem, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item.ID = strings.ToLower(strings.TrimSpace(item.ID))
		item.Name = strings.TrimSpace(item.Name)
		item.Symbol = strings.ToUpper(strings.TrimSpace(item.Symbol))
		if provider == "bitget" && strings.HasSuffix(strings.TrimSuffix(item.Symbol, quoteAsset), "ON") {
			item.Symbol = strings.TrimSuffix(strings.TrimSuffix(item.Symbol, quoteAsset), "ON") + quoteAsset
		}
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
	if v := os.Getenv("MARKETPULSE_MYSQL_ENABLED"); v != "" {
		c.MySQL.Enabled = parseEnvBool(v)
	}
	if v := os.Getenv("MARKETPULSE_MYSQL_HOST"); v != "" {
		c.MySQL.Host = v
	}
	if v := os.Getenv("MARKETPULSE_MYSQL_PORT"); v != "" {
		if n, err := parseEnvInt(v); err == nil {
			c.MySQL.Port = n
		}
	}
	if v := os.Getenv("MARKETPULSE_MYSQL_USER"); v != "" {
		c.MySQL.User = v
	}
	if v := os.Getenv("MARKETPULSE_MYSQL_PASSWORD"); v != "" {
		c.MySQL.Password = v
	}
	if v := os.Getenv("MARKETPULSE_MYSQL_DATABASE"); v != "" {
		c.MySQL.Database = v
	}
	if v := os.Getenv("MARKETPULSE_REDIS_ENABLED"); v != "" {
		c.Redis.Enabled = parseEnvBool(v)
	}
	if v := os.Getenv("MARKETPULSE_REDIS_ADDR"); v != "" {
		c.Redis.Addr = v
	}
	if v := os.Getenv("MARKETPULSE_REDIS_PASSWORD"); v != "" {
		c.Redis.Password = v
	}
	if v := os.Getenv("MARKETPULSE_REDIS_DB"); v != "" {
		if n, err := parseEnvInt(v); err == nil {
			c.Redis.DB = n
		}
	}
	if v := os.Getenv("MARKETPULSE_USERS_ENABLED"); v != "" {
		c.Users.Enabled = parseEnvBool(v)
	}
	if v := os.Getenv("MARKETPULSE_USERS_SEED_PHONE"); v != "" {
		c.Users.Seed.Phone = v
	}
	if v := os.Getenv("MARKETPULSE_USERS_SEED_PASSWORD"); v != "" {
		c.Users.Seed.Password = v
	}
	if v := os.Getenv("MARKETPULSE_USERS_SEED_NAME"); v != "" {
		c.Users.Seed.DisplayName = v
	}
}

func parseEnvBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseEnvInt(v string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
	return n, err
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
	switch c.Alpha.Provider {
	case "", "bitget", "binance":
	default:
		return fmt.Errorf("config: alpha.provider must be bitget or binance, got %q", c.Alpha.Provider)
	}
	if c.MySQL.Enabled {
		if c.MySQL.Database == "" {
			return fmt.Errorf("config: mysql.database is required when mysql.enabled")
		}
		if c.MySQL.Host == "" {
			return fmt.Errorf("config: mysql.host is required when mysql.enabled")
		}
	}
	if c.Redis.Enabled {
		if c.Redis.Addr == "" {
			return fmt.Errorf("config: redis.addr is required when redis.enabled")
		}
		if c.Redis.DB < 0 {
			return fmt.Errorf("config: redis.db must be >= 0")
		}
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
