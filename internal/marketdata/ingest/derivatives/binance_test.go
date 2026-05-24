package derivatives

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchGlobalLongShort(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("symbol"); got != "BTCUSDT" {
			t.Fatalf("symbol = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`[
			{"symbol":"BTCUSDT","longShortRatio":"1.2345","longAccount":"0.5525","shortAccount":"0.4475","timestamp":1710000000000}
		]`)),
		}, nil
	})}

	old := binanceLongShortURL
	binanceLongShortURL = "https://example.test/futures/data/globalLongShortAccountRatio"
	defer func() { binanceLongShortURL = old }()

	got, err := FetchGlobalLongShort(client, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ratio != 1.2345 {
		t.Fatalf("ratio = %v", got.Ratio)
	}
	if got.LongAccountPct != 55.25 || got.ShortAccountPct != 44.75 {
		t.Fatalf("accounts = %+v", got)
	}
	if got.UpdatedAt.Year() != 2024 {
		t.Fatalf("updatedAt = %s", got.UpdatedAt)
	}
}

func TestFetchTopLongShortPosition(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("symbol"); got != "BTCUSDT" {
			t.Fatalf("symbol = %q", got)
		}
		return jsonResponse(`[
			{"symbol":"BTCUSDT","longShortRatio":"1.2131","longAccount":"0.5482","shortAccount":"0.4518","timestamp":1710000000000}
		]`), nil
	})}

	old := binanceTopLongShortURL
	binanceTopLongShortURL = "https://example.test/futures/data/topLongShortPositionRatio"
	defer func() { binanceTopLongShortURL = old }()

	got, err := FetchTopLongShortPosition(client, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ratio != 1.2131 {
		t.Fatalf("ratio = %v", got.Ratio)
	}
	if got.LongAccountPct != 54.82 || got.ShortAccountPct != 45.18 {
		t.Fatalf("accounts = %+v", got)
	}
}

func TestFetchFunding(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("symbol"); got != "BTCUSDT" {
			t.Fatalf("symbol = %q", got)
		}
		return jsonResponse(`{
			"symbol":"BTCUSDT",
			"markPrice":"70100",
			"indexPrice":"70000",
			"lastFundingRate":"0.00012600",
			"nextFundingTime":1710028800000,
			"time":1710000000000
		}`), nil
	})}

	old := binancePremiumIndexURL
	binancePremiumIndexURL = "https://example.test/fapi/v1/premiumIndex"
	defer func() { binancePremiumIndexURL = old }()

	got, err := FetchFunding(client, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got.Rate != 0.000126 {
		t.Fatalf("rate = %v", got.Rate)
	}
	if got.NextFundingTime.IsZero() {
		t.Fatalf("next funding is zero")
	}
	if got.PremiumPct <= 0 || got.PremiumPct >= 0.2 {
		t.Fatalf("premiumPct = %v", got.PremiumPct)
	}
}

func TestFetchOpenInterest(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[
			{"symbol":"BTCUSDT","sumOpenInterest":"100","sumOpenInterestValue":"1000000","timestamp":"1710000000000"},
			{"symbol":"BTCUSDT","sumOpenInterest":"102","sumOpenInterestValue":"1100000","timestamp":1710003600000}
		]`), nil
	})}

	old := binanceOpenInterestHistURL
	binanceOpenInterestHistURL = "https://example.test/futures/data/openInterestHist"
	defer func() { binanceOpenInterestHistURL = old }()

	got, err := FetchOpenInterest(client, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got.ValueUsd != 1100000 {
		t.Fatalf("value = %v", got.ValueUsd)
	}
	if got.ChangePct != 10 {
		t.Fatalf("change = %v", got.ChangePct)
	}
}

func TestFetchTakerBuySell(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`[
			{"buySellRatio":"1.18","buyVol":"16800000000","sellVol":"14200000000","timestamp":"1710000000000"}
		]`), nil
	})}

	old := binanceTakerBuySellURL
	binanceTakerBuySellURL = "https://example.test/futures/data/takerlongshortRatio"
	defer func() { binanceTakerBuySellURL = old }()

	got, err := FetchTakerBuySell(client, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ratio != 1.18 || got.BuyVol != 16800000000 || got.SellVol != 14200000000 {
		t.Fatalf("taker = %+v", got)
	}
}

func TestParseLiquidationMessage(t *testing.T) {
	raw := []byte(`{
		"e":"forceOrder",
		"E":1710000000000,
		"o":{"s":"BTCUSDT","S":"SELL","q":"0.50","p":"70000","ap":"70100","z":"0.40"}
	}`)
	got, ok := ParseLiquidationMessage(raw)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.Symbol != "BTCUSDT" || got.Side != "SELL" {
		t.Fatalf("order = %+v", got)
	}
	if got.Notional != 28040 {
		t.Fatalf("notional = %v", got.Notional)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
