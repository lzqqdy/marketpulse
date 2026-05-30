package bitget

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
)

func TestParseTickerNormalizesChangeRatio(t *testing.T) {
	tick, ok := ParseTicker(tickerRaw{
		Symbol:      "AAPLUSDT",
		LastPr:      "312.27",
		Change24h:   "-0.03298",
		BaseVolume:  "10",
		USDTVolume:  "3122.7",
		MarkPrice:   "312.20",
		IndexPrice:  "312.10",
		FundingRate: "0.0001",
		TS:          "1780103672414",
	})
	if !ok {
		t.Fatal("ticker not parsed")
	}
	if tick.Symbol != "AAPLUSDT" || tick.Price != 312.27 || tick.Change24hPct != -3.298 || tick.Volume != 3122.7 {
		t.Fatalf("ticker = %+v", tick)
	}
	if tick.MarkPrice != 312.20 || tick.IndexPrice != 312.10 || tick.FundingRate != 0.0001 {
		t.Fatalf("extended fields = %+v", tick)
	}
	if !tick.UpdatedAt.Equal(time.UnixMilli(1780103672414).UTC()) {
		t.Fatalf("updatedAt = %s", tick.UpdatedAt)
	}
}

func TestParseCandleRows(t *testing.T) {
	candles := ParseCandleRows([][]string{
		{"1780103520000", "73560.1", "73560.1", "73548", "73548", "1.7551", "129102.00337"},
	})
	if len(candles) != 1 {
		t.Fatalf("candles len = %d", len(candles))
	}
	c := candles[0]
	if c.Time != 1780103520 || c.Open != 73560.1 || c.Close != 73548 || c.Volume != 1.7551 || c.QuoteVolume != 129102.00337 || !c.Closed {
		t.Fatalf("candle = %+v", c)
	}
}

func TestGranularitySupportsUIIntervals(t *testing.T) {
	cases := map[string]string{
		"1m":  "1m",
		"5m":  "5m",
		"15m": "15m",
		"1h":  "1H",
		"4h":  "4H",
		"1d":  "1D",
		"1w":  "1W",
	}
	for input, want := range cases {
		got, err := Granularity(input)
		if err != nil || got != want {
			t.Fatalf("Granularity(%s) = %s, %v", input, got, err)
		}
	}
}

func TestCandleChannelOnlySupportsRequestedWSIntervals(t *testing.T) {
	if ch, err := CandleChannel("1m"); err != nil || ch != "candle1m" {
		t.Fatalf("1m channel = %s, %v", ch, err)
	}
	if ch, err := CandleChannel("1h"); err != nil || ch != "candle1H" {
		t.Fatalf("1h channel = %s, %v", ch, err)
	}
	if _, err := CandleChannel("1d"); err == nil {
		t.Fatal("expected 1d ws channel to be unsupported")
	}
}

func TestParseWSMessageTickerAndCandle(t *testing.T) {
	tickers, candles, ok := ParseWSMessage([]byte(`{"action":"snapshot","arg":{"instType":"USDT-FUTURES","channel":"ticker","instId":"AAPLUSDT"},"data":[{"symbol":"AAPLUSDT","lastPr":"312.27","change24h":"0.01","usdtVolume":"100","ts":"1780103672414"}]}`))
	if !ok || len(tickers) != 1 || len(candles) != 0 {
		t.Fatalf("ticker ws parsed = tickers:%+v candles:%+v ok:%v", tickers, candles, ok)
	}
	if tickers[0].Change24hPct != 1 {
		t.Fatalf("ticker change = %+v", tickers[0])
	}

	tickers, candles, ok = ParseWSMessage([]byte(`{"action":"snapshot","arg":{"instType":"USDT-FUTURES","channel":"candle1m","instId":"AAPLUSDT"},"data":[["1780103520000","1","2","0.5","1.5","10","15"]]}`))
	if !ok || len(tickers) != 0 || len(candles) != 1 {
		t.Fatalf("candle ws parsed = tickers:%+v candles:%+v ok:%v", tickers, candles, ok)
	}
	if candles[0].Closed {
		t.Fatalf("ws candle should be open: %+v", candles[0])
	}
}

func TestResolveItemsSkipsMissingSymbols(t *testing.T) {
	rows := []tickerRaw{
		{Symbol: "AAPLUSDT", LastPr: "312.27", Change24h: "0.01", USDTVolume: "100"},
	}
	tickers := parseTickers(rows)
	available := make(map[string]struct{}, len(tickers))
	for symbol := range tickers {
		available[symbol] = struct{}{}
	}
	items := []config.AlphaItem{
		{ID: "aapl", Name: "AAPL", Symbol: "AAPLUSDT"},
		{ID: "fake", Name: "FAKE", Symbol: "FAKEUSDT"},
	}
	resolved, missing := resolveItemsFromAvailable(items, nil, "USDT", available)
	if len(resolved) != 1 || resolved[0].Symbol != "AAPLUSDT" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if len(missing) != 1 || missing[0].Symbol != "FAKEUSDT" {
		t.Fatalf("missing = %+v", missing)
	}
}
