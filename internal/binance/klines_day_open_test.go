package binance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchExchangeDayOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("interval") != "1d" {
			t.Fatalf("interval: %s", r.URL.Query().Get("interval"))
		}
		row := []interface{}{
			int64(1715817600000), "77000.0", "78000", "76000", "77500", "100",
		}
		_ = json.NewEncoder(w).Encode([][]interface{}{row})
	}))
	defer srv.Close()

	old := restBase
	restBase = srv.URL
	t.Cleanup(func() { restBase = old })

	open, err := FetchExchangeDayOpen("BTC")
	if err != nil || open != 77000 {
		t.Fatalf("open=%v err=%v", open, err)
	}
}
