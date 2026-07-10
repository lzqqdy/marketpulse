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
	if !cfg.Alpha.Enabled || len(cfg.Alpha.Indices) != 2 || len(cfg.Alpha.Stocks) != 7 {
		t.Fatalf("alpha config: enabled=%v indices=%d stocks=%d", cfg.Alpha.Enabled, len(cfg.Alpha.Indices), len(cfg.Alpha.Stocks))
	}
	if cfg.Alpha.Provider != "bitget" || cfg.Alpha.ProductType != "USDT-FUTURES" {
		t.Fatalf("alpha provider: provider=%s productType=%s", cfg.Alpha.Provider, cfg.Alpha.ProductType)
	}
	if cfg.Alpha.PollInterval != 30*time.Second || cfg.Alpha.ResolveInterval != 10*time.Minute {
		t.Fatalf("alpha intervals: poll=%s resolve=%s", cfg.Alpha.PollInterval, cfg.Alpha.ResolveInterval)
	}
	if strings.Join(cfg.AlphaBaseSymbols(), ",") != "QQQ,SPY,AAPL,MSFT,NVDA,AMZN,GOOGL,META,TSLA" {
		t.Fatalf("alpha base symbols: %v", cfg.AlphaBaseSymbols())
	}
	if strings.Join(cfg.DayOpenSymbols(), ",") != strings.Join(cfg.Symbols, ",") {
		t.Fatalf("day open symbols should exclude alpha: %v", cfg.DayOpenSymbols())
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
