package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/expressnews"
)

func testTime() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) }

func equityStub() *stubMD {
	now := testTime()
	return &stubMD{
		indexByID: map[string]marketdata.IndexQuote{
			"us-aapl": {ID: "us-aapl", Name: "苹果", Price: 200, ChangePct: 1, Source: "tencent", UpdatedAt: now, FetchedAt: now},
			"us-nvda": {ID: "us-nvda", Name: "英伟达", Price: 150, ChangePct: -2, Source: "baidu", UpdatedAt: now, FetchedAt: now},
		},
		alphaByID: map[string]marketdata.AlphaQuote{
			"aapl": {ID: "aapl", Name: "苹果", Symbol: "AAPL", Price: 202, Change24hPct: 3, ChangeDayPct: 99, Source: "bitget", UpdatedAt: now},
		},
		indexKlines: marketdata.KlineResponse{Symbol: "us-aapl", Pair: "105.AAPL", Source: "baidu", Interval: "1d",
			Candles: []binance.Candle{{Time: now.Add(-24 * time.Hour).Unix(), Open: 190, Close: 200, High: 201, Low: 189, Closed: true}}},
		klines: marketdata.KlineResponse{Symbol: "AAPL", Pair: "AAPLUSDT", Source: "bitget", Interval: "1d",
			Candles: []binance.Candle{{Time: now.Add(-24 * time.Hour).Unix(), Open: 191, Close: 202, High: 203, Low: 190, Closed: true}}},
		newsResponse: &expressnews.Response{Source: "baidu", FetchedAt: now.Unix(), Items: []expressnews.NewsItem{
			{ID: "apple", Title: "苹果发布公告", PublishTime: now.Unix(), ThirdURL: "https://example.com/apple"},
		}},
	}
}

func executeTest(t *testing.T, reg *Registry, name, args string) map[string]any {
	t.Helper()
	raw, err := reg.Execute(context.Background(), name, args)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestUSEquityAndContractIdentity(t *testing.T) {
	for _, tc := range []struct {
		args, asset, kind, currency, basis string
		price                              float64
	}{
		{`{"symbol":"AAPL"}`, "us_equity", "equity_spot", "USD", "trading_session", 200},
		{`{"symbol":"苹果"}`, "us_equity", "equity_spot", "USD", "trading_session", 200},
		{`{"symbol":"us-aapl","assetClass":"index"}`, "us_equity", "equity_spot", "USD", "trading_session", 200},
		{`{"symbol":"AAPLUSDT"}`, "us_contract", "usdt_futures", "USDT", "rolling_24h", 202},
		{`{"symbol":"AAPL","assetClass":"alpha"}`, "us_contract", "usdt_futures", "USDT", "rolling_24h", 202},
	} {
		t.Run(tc.args, func(t *testing.T) {
			reg := NewRegistry(equityStub())
			reg.now = testTime
			out := executeTest(t, reg, "get_quote", tc.args)
			if out["ok"] != true || out["assetClass"] != tc.asset || out["instrumentType"] != tc.kind || out["currency"] != tc.currency || out["changeBasis"] != tc.basis || out["price"] != tc.price {
				t.Fatalf("wrong instrument semantics: %v", out)
			}
			if tc.asset == "us_contract" && out["changeDayPct"] != nil {
				t.Fatal("contract exposed a stock-session change")
			}
			rows := out["evidence"].([]any)
			record := rows[0].(map[string]any)
			if record["currency"] != tc.currency || record["asOf"] == nil || record["source"] == "" {
				t.Fatalf("missing evidence identity: %v", record)
			}
		})
	}
}

func TestMissingSpotNeverSubstitutesContract(t *testing.T) {
	stub := equityStub()
	delete(stub.indexByID, "us-aapl")
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_quote", `{"symbol":"AAPL"}`)
	if out["ok"] != false || out["status"] != "unavailable" || out["price"] != nil {
		t.Fatalf("contract substituted for spot: %v", out)
	}
	if executeTest(t, reg, "get_quote", `{"symbol":"AAPLUSDT","assetClass":"us_equity"}`)["ok"] != false {
		t.Fatal("conflicting identity accepted")
	}
}

func TestCryptoTickerIsNotConfusedWithMainlandIndexPrefix(t *testing.T) {
	stub := equityStub()
	stub.quoteBySymbol = map[string]marketdata.Quote{"SHIB": {Symbol: "SHIB", PriceUsdt: 0.00001, UpdatedAt: testTime()}}
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_quote", `{"symbol":"SHIB"}`)
	if out["assetClass"] != "crypto" || out["ok"] != true {
		t.Fatalf("%v", out)
	}
}

func TestStaleMissingAndClosedMarketQuoteTimes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		at, fetched time.Time
		stale       bool
		status      string
	}{
		{"fresh", testTime(), testTime(), false, "ok"},
		{"closed-session", testTime().Add(-72 * time.Hour), testTime(), false, "ok"},
		{"failed-refresh", testTime(), testTime().Add(-2 * time.Hour), false, "stale"},
		{"missing-time", time.Time{}, testTime(), false, "stale"},
		{"provider-stale", testTime(), testTime(), true, "stale"},
		{"future-time", testTime().Add(2 * time.Minute), testTime(), false, "stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := equityStub()
			q := stub.indexByID["us-aapl"]
			q.UpdatedAt, q.FetchedAt, q.Stale = tc.at, tc.fetched, tc.stale
			stub.indexByID["us-aapl"] = q
			reg := NewRegistry(stub)
			reg.now = testTime
			out := executeTest(t, reg, "get_quote", `{"symbol":"AAPL"}`)
			if out["status"] != tc.status {
				t.Fatalf("%v", out)
			}
		})
	}
}

func TestKlineRoutingAndNoCrossAssetFallback(t *testing.T) {
	for _, tc := range []struct{ args, call string }{
		{`{"symbol":"AAPL"}`, "index:us-aapl"},
		{`{"symbol":"AAPL","assetClass":"us_contract"}`, "reference:AAPL"},
	} {
		stub := equityStub()
		reg := NewRegistry(stub)
		reg.now = testTime
		out := executeTest(t, reg, "get_klines_summary", tc.args)
		if out["ok"] != true || len(stub.klineCalls) != 1 || stub.klineCalls[0] != tc.call {
			t.Fatalf("wrong route: %v %v", out, stub.klineCalls)
		}
	}
	stub := equityStub()
	stub.indexKlineErr = errors.New("spot history unavailable")
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_klines_summary", `{"symbol":"AAPL"}`)
	if out["status"] != "unavailable" || len(stub.klineCalls) != 1 {
		t.Fatalf("cross-asset fallback: %v %v", out, stub.klineCalls)
	}
	stub = equityStub()
	stub.indexKlines.Symbol = "us-nvda"
	if executeTest(t, NewRegistry(stub), "get_klines_summary", `{"symbol":"AAPL"}`)["ok"] != false {
		t.Fatal("another symbol's history accepted")
	}
	stub = equityStub()
	stub.klines.Source = "bitget"
	if executeTest(t, NewRegistry(stub), "get_klines_summary", `{"symbol":"AAPL","assetClass":"crypto"}`)["ok"] != false {
		t.Fatal("reference source relabeled as crypto spot")
	}
	stub = equityStub()
	stub.klines.Source = "binance"
	if executeTest(t, NewRegistry(stub), "get_klines_summary", `{"symbol":"AAPL","assetClass":"us_contract"}`)["ok"] != false {
		t.Fatal("spot source relabeled as a contract")
	}
	stub = equityStub()
	stub.indexKlines.Candles[0].Close = 0
	if executeTest(t, NewRegistry(stub), "get_klines_summary", `{"symbol":"AAPL"}`)["ok"] != false {
		t.Fatal("zero-price candles treated as valid history")
	}
}

func TestActiveMarketObservationExpiresSooner(t *testing.T) {
	stub := equityStub()
	now := testTime().Add(2 * time.Hour)
	q := stub.indexByID["us-aapl"]
	q.UpdatedAt, q.FetchedAt = now.Add(-3*time.Minute), now.Add(-3*time.Minute)
	stub.indexByID["us-aapl"] = q
	reg := NewRegistry(stub)
	reg.now = func() time.Time { return now }
	if out := executeTest(t, reg, "get_quote", `{"symbol":"AAPL"}`); out["status"] != "stale" {
		t.Fatalf("%v", out)
	}
}

func TestReferenceFallbackRemainsTokenizedNotFutures(t *testing.T) {
	stub := equityStub()
	stub.klines.Source = "binance-alpha"
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_symbol_brief", `{"symbol":"AAPL","assetClass":"us_contract"}`)
	if out["status"] != "partial" {
		t.Fatalf("%v", out)
	}
	kline := out["kline1d"].(map[string]any)
	if kline["instrumentType"] != "tokenized_equity" || out["limitations"] == nil {
		t.Fatalf("fallback mislabeled: %v", out)
	}
}

func TestMixedQuoteBasesAreNotRankedAsComparable(t *testing.T) {
	stub := equityStub()
	stub.quoteBySymbol = map[string]marketdata.Quote{"BTC": {Symbol: "BTC", PriceUsdt: 100000, Change24hPct: 2, UpdatedAt: testTime()}}
	stub.klines.Symbol, stub.klines.Source = "BTC", "binance"
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "compare_symbols", `{"symbols":["BTC","AAPL"]}`)
	if out["status"] != "partial" || out["limitations"] == nil {
		t.Fatalf("mixed bases presented as comparable: %v", out)
	}
}

func TestSymbolNewsMatching(t *testing.T) {
	stub := equityStub()
	stub.newsResponse.Items = []expressnews.NewsItem{
		{ID: "btc", Title: "BTC突破", PublishTime: testTime().Unix()},
		{ID: "unrelated", Title: "苹果供应链", PublishTime: testTime().Unix()},
		{ID: "entity", Title: "公司发布公告", PublishTime: testTime().Unix(), Entities: []expressnews.NewsEntity{{Code: "NASDAQ:NVDA", Market: "us"}}},
		{ID: "name", Title: "英伟达发布新产品", PublishTime: testTime().Unix()},
		{ID: "ticker", Title: "NVDA上涨", PublishTime: testTime().Unix()},
		{ID: "ticker", Title: "NVDA上涨", PublishTime: testTime().Unix()},
	}
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_express_news", `{"symbol":"NVDA","tag":"币圈","limit":3}`)
	if stub.newsTag != "美股" || stub.newsRN != 20 {
		t.Fatalf("wrong news market: %s %d", stub.newsTag, stub.newsRN)
	}
	items := out["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["matchedBy"] != "entity" {
		t.Fatalf("wrong matching: %v", items)
	}
	meta, _ := reg.resolveInstrument("META", "us_equity")
	if matchNews(expressnews.NewsItem{Title: "metaverse markets"}, meta) != "" {
		t.Fatal("META matched a substring")
	}
	if matchNews(expressnews.NewsItem{Title: "META 发布业绩"}, meta) != "text" {
		t.Fatal("complete ticker not matched")
	}
}

func TestNoMatchingNewsIsPartialNotInvented(t *testing.T) {
	stub := equityStub()
	stub.newsResponse.Items = []expressnews.NewsItem{{Title: "BTC 新闻"}}
	reg := NewRegistry(stub)
	reg.now = testTime
	out := executeTest(t, reg, "get_symbol_brief", `{"symbol":"AAPL"}`)
	news := out["news"].(map[string]any)
	if stub.newsTag != "美股" || out["status"] != "partial" || len(news["items"].([]any)) != 0 || news["hint"] == nil {
		t.Fatalf("%v", out)
	}
}

func TestOutcomeBusinessFailuresAndValidJSON(t *testing.T) {
	for _, raw := range []string{`{"ok":false}`, `{"ok":false,"status":"ok"}`, `not JSON`, `{"ok":true,"status":"unavailable"}`} {
		if got := ParseOutcome(raw, nil); got.OK || got.Status != evidence.Unavailable {
			t.Fatalf("promoted failure: %v", got)
		}
	}
	if ParseOutcome(`{"ok":true}`, errors.New("timeout")).OK {
		t.Fatal("transport error ignored")
	}
	if got := ParseOutcome(`{"ok":false,"status":"partial"}`, nil); got.OK || got.Status != evidence.Partial {
		t.Fatalf("%v", got)
	}
	partial := ParseOutcome(`{"ok":false,"status":"partial","quote":{"status":"unavailable","error":"spot missing"}}`, nil)
	if !strings.Contains(partial.Summary, "spot missing") {
		t.Fatal("child failure reason was lost")
	}
	reg := NewRegistry(equityStub())
	reg.now = testTime
	raw, err := reg.Execute(context.Background(), "get_symbol_brief", `{"symbol":"AAPL"}`)
	if err != nil {
		t.Fatal(err)
	}
	got := ParseOutcome(raw, nil)
	if !got.OK || len(got.Evidence) != 3 {
		t.Fatalf("%v", got)
	}
	encoded, _ := json.Marshal(got.Evidence)
	if !strings.Contains(string(encoded), "https://example.com/apple") || len(encoded) <= 280 {
		t.Fatalf("evidence missing: %s", encoded)
	}
}
