package binance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchKlineOpenAt(t *testing.T) {
	start := time.Date(2026, 5, 16, 0, 0, 0, 0, Shanghai)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("startTime") == "" {
			t.Fatal("missing startTime")
		}
		if r.URL.Query().Get("interval") != "1m" {
			t.Fatal("expected 1m")
		}
		row := []interface{}{
			start.UnixMilli(), "50000.0", "50100", "49900", "50050", "10",
		}
		_ = json.NewEncoder(w).Encode([][]interface{}{row})
	}))
	defer srv.Close()

	old := restBase
	restBase = srv.URL
	t.Cleanup(func() { restBase = old })

	open, err := FetchKlineOpenAt("BTC", start)
	if err != nil || open != 50000 {
		t.Fatalf("open=%v err=%v", open, err)
	}
}
