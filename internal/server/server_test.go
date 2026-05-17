package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/hub"
	"github.com/lzqqdy/marketpulse/internal/ingest"
	"github.com/lzqqdy/marketpulse/internal/store"
)

func TestHealthz_andSnapshot(t *testing.T) {
	cfg := &config.Config{
		App:     config.AppConfig{Addr: ":8080", Mode: "debug"},
		Symbols: []string{"BTC"},
	}
	st := store.New("BTC")
	st.UpdateQuote(store.Quote{Symbol: "BTC", PriceUsdt: 1})

	srv := New(Deps{
		Config:    cfg,
		Store:     st,
		StreamHub: hub.NewStreamHub(st),
		KlineHub:  hub.NewKlineHub(cfg),
		Ingest:    ingest.New(cfg, st),
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz: %d %s", w.Code, w.Body.String())
	}

	var health map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if health["symbolCount"].(float64) != 1 {
		t.Fatalf("symbolCount: %v", health["symbolCount"])
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/snapshot", nil)
	srv.Engine().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("snapshot: %d", w2.Code)
	}
}
