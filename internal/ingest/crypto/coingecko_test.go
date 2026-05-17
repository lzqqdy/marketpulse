package crypto

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchMarketMetadata(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("ids"); got != "bitcoin,ethereum" {
			t.Fatalf("ids = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`[
			{"id":"bitcoin","symbol":"btc","image":"https://img/btc.png","market_cap_rank":1,"market_cap":1900000000000,"total_volume":42000000000},
			{"id":"ethereum","symbol":"eth","image":"https://img/eth.png","market_cap_rank":2,"market_cap":420000000000,"total_volume":16000000000}
		]`)),
		}, nil
	})}

	old := coinGeckoMarketsURL
	coinGeckoMarketsURL = "https://example.test/coins/markets"
	defer func() { coinGeckoMarketsURL = old }()

	rows, err := FetchMarketMetadata(client, []string{"BTC", "ETH", "UNKNOWN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Symbol != "BTC" || rows[0].Rank != 1 || rows[0].MarketCapUsd != 1900000000000 {
		t.Fatalf("btc row = %+v", rows[0])
	}
	if !strings.Contains(rows[1].IconURL, "eth") {
		t.Fatalf("eth icon = %q", rows[1].IconURL)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
