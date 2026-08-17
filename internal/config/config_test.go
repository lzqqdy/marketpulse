package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_example(t *testing.T) {
	path := filepath.Join("..", "..", "config", "config.example.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Symbols) < 5 {
		t.Fatalf("expected at least 5 symbols, got %v", cfg.Symbols)
	}
	if cfg.App.Addr != ":8080" {
		t.Fatalf("addr: %s", cfg.App.Addr)
	}
	if cfg.Ingest.OTC.USDTCNYInterval != 30*time.Second {
		t.Fatalf("otc interval: %s", cfg.Ingest.OTC.USDTCNYInterval)
	}
	if !strings.Contains(cfg.BinanceStreamURL(), "btcusdt@miniTicker") {
		t.Fatalf("stream url: %s", cfg.BinanceStreamURL())
	}
	if strings.Join(cfg.Ingest.Equity.Providers, ",") != "baidu,tencent,eastmoney" {
		t.Fatalf("providers: %v", cfg.Ingest.Equity.Providers)
	}
	if !cfg.Ingest.Baidu.IsEnabled() || !cfg.Ingest.Baidu.IsWSEnabled() {
		t.Fatalf("baidu defaults: enabled=%v ws=%v", cfg.Ingest.Baidu.IsEnabled(), cfg.Ingest.Baidu.IsWSEnabled())
	}
	if !cfg.Alpha.Enabled || len(cfg.Alpha.Indices) != 2 || len(cfg.Alpha.Stocks) != 10 {
		t.Fatalf("alpha config: enabled=%v indices=%d stocks=%d", cfg.Alpha.Enabled, len(cfg.Alpha.Indices), len(cfg.Alpha.Stocks))
	}
	if cfg.Alpha.Provider != "bitget" || cfg.Alpha.ProductType != "USDT-FUTURES" {
		t.Fatalf("alpha provider: provider=%s productType=%s", cfg.Alpha.Provider, cfg.Alpha.ProductType)
	}
	if cfg.Alpha.PollInterval != 30*time.Second || cfg.Alpha.ResolveInterval != 10*time.Minute {
		t.Fatalf("alpha intervals: poll=%s resolve=%s", cfg.Alpha.PollInterval, cfg.Alpha.ResolveInterval)
	}
	if strings.Join(cfg.AlphaBaseSymbols(), ",") != "QQQ,SPY,AAPL,MSFT,NVDA,AMZN,GOOGL,META,TSLA,MU,TSM,SKHY" {
		t.Fatalf("alpha base symbols: %v", cfg.AlphaBaseSymbols())
	}
	if !containsString(cfg.Ingest.Equity.IndexIDs, "us-tsm") || !containsString(cfg.Ingest.Equity.IndexIDs, "us-skhy") {
		t.Fatalf("equity index_ids missing us equities: %v", cfg.Ingest.Equity.IndexIDs)
	}
	if strings.Join(cfg.DayOpenSymbols(), ",") != strings.Join(cfg.Symbols, ",") {
		t.Fatalf("day open symbols should exclude alpha: %v", cfg.DayOpenSymbols())
	}
}

func TestLoad_wxcloudrunYaml(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("MYSQL_ADDRESS", "")
	t.Setenv("MYSQL_USERNAME", "")
	t.Setenv("MYSQL_PASSWORD", "")
	t.Setenv("MYSQL_DATABASE", "")
	cfg, err := Load(filepath.Join("..", "..", "config", "config.wxcloudrun.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Addr != "0.0.0.0:80" {
		t.Fatalf("addr: %s", cfg.App.Addr)
	}
	if cfg.App.StaticDir != "web/dist" {
		t.Fatalf("static_dir: %s", cfg.App.StaticDir)
	}
	if !cfg.MySQL.Enabled || !cfg.Redis.Embedded || !cfg.Users.Enabled || !cfg.Alerts.Enabled || !cfg.Portfolio.Enabled || !cfg.Event.Enabled {
		t.Fatalf("wxcloudrun modules: mysql=%v redis.embedded=%v users=%v alerts=%v portfolio=%v event=%v",
			cfg.MySQL.Enabled, cfg.Redis.Embedded, cfg.Users.Enabled, cfg.Alerts.Enabled, cfg.Portfolio.Enabled, cfg.Event.Enabled)
	}
	if cfg.AI.Enabled {
		t.Fatal("ai should stay off without api key")
	}
	if !strings.Contains(cfg.Ingest.Binance.WSBase, "binance.vision") {
		t.Fatalf("binance ws: %s", cfg.Ingest.Binance.WSBase)
	}
}

func TestLoad_envOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
app:
  addr: ":9000"
  mode: "release"
symbols:
  - BTC
ingest:
  binance:
    ws_base: "wss://example.test/stream"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARKETPULSE_APP_ADDR", ":7777")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Addr != ":7777" {
		t.Fatalf("env addr: %s", cfg.App.Addr)
	}
	if cfg.App.Mode != "release" {
		t.Fatalf("mode: %s", cfg.App.Mode)
	}
}

func TestLoad_bitgetAlphaMigratesOndoSymbols(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
app:
  mode: debug
symbols:
  - BTC
alpha:
  enabled: true
  provider: bitget
  quote_asset: USDT
  indices:
    - id: qqqon
      name: QQQ
      symbol: QQQONUSDT
  stocks:
    - id: aaplon
      name: AAPL
      symbol: AAPLONUSDT
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.AlphaBaseSymbols(), ",") != "QQQ,AAPL" {
		t.Fatalf("alpha base symbols: %v", cfg.AlphaBaseSymbols())
	}
	if cfg.Alpha.Indices[0].Symbol != "QQQUSDT" || cfg.Alpha.Stocks[0].Symbol != "AAPLUSDT" {
		t.Fatalf("alpha symbols: indices=%+v stocks=%+v", cfg.Alpha.Indices, cfg.Alpha.Stocks)
	}
	if cfg.Alpha.Indices[0].ID != "qqq" || cfg.Alpha.Stocks[0].ID != "aapl" {
		t.Fatalf("alpha ids should strip on suffix: indices=%+v stocks=%+v", cfg.Alpha.Indices, cfg.Alpha.Stocks)
	}
}

func TestLoad_emptySymbols(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte("app:\n  mode: debug\nsymbols: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty symbols")
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
