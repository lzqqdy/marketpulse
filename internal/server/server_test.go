package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

func TestHealthz_andSnapshot(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{Addr: ":8080", Mode: "debug"},
		Symbols: []string{"BTC"},
	}
	st := store.New("BTC")
	st.UpdateQuote(store.Quote{Symbol: "BTC", PriceUsdt: 1})
	marketData := marketdata.NewWithStore(cfg, st)

	srv := New(Deps{
		Config:     cfg,
		MarketData: marketData,
	})

	for _, path := range []string{"/healthz", "/health"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		srv.Engine().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var health map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
			t.Fatal(err)
		}
		if health["symbolCount"].(float64) != 1 {
			t.Fatalf("%s symbolCount: %v", path, health["symbolCount"])
		}
	}

	for _, path := range []string{"/api/v1/market/snapshot", "/api/v1/snapshot"} {
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, path, nil)
		srv.Engine().ServeHTTP(w2, req2)
		if w2.Code != http.StatusOK {
			t.Fatalf("snapshot %s: %d", path, w2.Code)
		}
	}
}
