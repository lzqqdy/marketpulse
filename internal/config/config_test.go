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
	if strings.Join(cfg.Ingest.Equity.Providers, ",") != "yahoo,twelvedata,stooq" {
		t.Fatalf("providers: %v", cfg.Ingest.Equity.Providers)
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
	t.Setenv("MARKETPULSE_FINNHUB_API_KEY", "fh_test")
	t.Setenv("MARKETPULSE_TWELVEDATA_API_KEY", "td_test")
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
	if cfg.Ingest.Equity.FinnhubAPIKey != "fh_test" {
		t.Fatalf("finnhub key: %s", cfg.Ingest.Equity.FinnhubAPIKey)
	}
	if cfg.Ingest.Equity.TwelveDataAPIKey != "td_test" {
		t.Fatalf("twelve key: %s", cfg.Ingest.Equity.TwelveDataAPIKey)
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
