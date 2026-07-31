package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/expressnews"
	"github.com/lzqqdy/marketpulse/internal/marketdata/marketcenter"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

type stubMD struct {
	quote   marketdata.Quote
	ok      bool
	newsRN  int
	klines  marketdata.KlineResponse
	center  marketcenter.CenterResponse
}

func (s *stubMD) Start(ctx context.Context)                              {}
func (s *stubMD) Snapshot() marketdata.Snapshot {
	return marketdata.Snapshot{
		Quotes: []store.Quote{s.quote, {Symbol: "ETH", Change24hPct: -2, PriceUsdt: 3000}},
		Macro:  store.MacroSnapshot{FearGreed: store.FearGreed{Value: 40, Label: "Fear"}},
	}
}
func (s *stubMD) Quote(symbol string) (marketdata.Quote, bool) {
	return s.quote, s.ok && s.quote.Symbol == symbol
}
func (s *stubMD) IndexQuote(id string) (marketdata.IndexQuote, bool) { return marketdata.IndexQuote{}, false }
func (s *stubMD) AlphaQuote(id string) (marketdata.AlphaQuote, bool) { return marketdata.AlphaQuote{}, false }
func (s *stubMD) AddListener(listener store.ChangeListener)          {}
func (s *stubMD) Version() uint64                                    { return 1 }
func (s *stubMD) SymbolCount() int                                   { return 1 }
func (s *stubMD) ProviderStatus() marketdata.ProviderStatusResponse  { return marketdata.ProviderStatusResponse{} }
func (s *stubMD) IngestStatus() map[string]string                    { return nil }
func (s *stubMD) StreamClientCount() int                             { return 0 }
func (s *stubMD) ServeStreamWS(conn *websocket.Conn, channels string) {}
func (s *stubMD) ServeKlineWS(conn *websocket.Conn, symbol, interval string) {
}
func (s *stubMD) Klines(symbol, interval string, limit int) (marketdata.KlineResponse, error) {
	return s.klines, nil
}
func (s *stubMD) IndexKlines(id, interval string, limit int) (marketdata.KlineResponse, error) {
	return marketdata.KlineResponse{}, nil
}
func (s *stubMD) MarketCenter(market string) (marketcenter.CenterResponse, error) {
	s.center.Market = market
	return s.center, nil
}
func (s *stubMD) MarketCenterHeatmap(market, sortKey string) (marketcenter.Heatmap, error) {
	return marketcenter.Heatmap{}, nil
}
func (s *stubMD) ExpressNews(tag string, pn, rn, filterByUserStocks int) (expressnews.Response, error) {
	s.newsRN = rn
	items := make([]expressnews.NewsItem, 0, rn)
	for i := 0; i < rn; i++ {
		items = append(items, expressnews.NewsItem{Title: "t", Body: "b"})
	}
	return expressnews.Response{Tag: tag, Items: items, Source: "stub"}, nil
}

func TestGetQuoteCrypto(t *testing.T) {
	reg := NewRegistry(&stubMD{
		ok: true,
		quote: marketdata.Quote{
			Symbol:       "BTC",
			PriceUsdt:    100000,
			Change24hPct: 1.5,
		},
	})
	out, err := reg.Execute(context.Background(), "get_quote", `{"symbol":"BTCUSDT"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", out)
	}
	if m["symbol"] != "BTC" {
		t.Fatalf("symbol=%v", m["symbol"])
	}
}

func TestNormalizeCryptoSymbol(t *testing.T) {
	if got := normalizeCryptoSymbol("btcusdt"); got != "BTC" {
		t.Fatalf("got %s", got)
	}
}

func TestExpressNewsLimit(t *testing.T) {
	stub := &stubMD{}
	reg := NewRegistry(stub)
	out, err := reg.Execute(context.Background(), "get_express_news", `{"limit":3,"tag":"币圈"}`)
	if err != nil {
		t.Fatal(err)
	}
	if stub.newsRN != 3 {
		t.Fatalf("rn=%d", stub.newsRN)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	items, _ := m["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items=%d", len(items))
	}
}

func TestKlinesSummary(t *testing.T) {
	reg := NewRegistry(&stubMD{
		klines: marketdata.KlineResponse{
			Symbol:   "BTC",
			Interval: "1d",
			Source:   "stub",
			Candles: []binance.Candle{
				{Time: 1, Open: 100, High: 110, Low: 90, Close: 105, Volume: 10},
				{Time: 2, Open: 105, High: 120, Low: 100, Close: 115, Volume: 12},
				{Time: 3, Open: 115, High: 130, Low: 110, Close: 125, Volume: 11},
				{Time: 4, Open: 125, High: 140, Low: 120, Close: 135, Volume: 13},
				{Time: 5, Open: 135, High: 150, Low: 130, Close: 145, Volume: 14},
				{Time: 6, Open: 145, High: 155, Low: 140, Close: 150, Volume: 15},
				{Time: 7, Open: 150, High: 160, Low: 145, Close: 158, Volume: 30},
			},
		},
	})
	out, err := reg.Execute(context.Background(), "get_klines_summary", `{"symbol":"BTC","interval":"1d"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if m["ok"] != true {
		t.Fatalf("%s", out)
	}
	if m["candleCount"].(float64) != 7 {
		t.Fatalf("count=%v", m["candleCount"])
	}
	if _, ok := m["rangeChangePct"]; !ok {
		t.Fatal("missing rangeChangePct")
	}
	if _, ok := m["sma7"]; !ok {
		t.Fatal("missing sma7")
	}
	if m["closeVsSma7"] != "above" && m["closeVsSma7"] != "below" {
		t.Fatalf("closeVsSma7=%v", m["closeVsSma7"])
	}
	if _, ok := m["volumeNote"]; !ok {
		t.Fatal("missing volumeNote")
	}
}

func TestCompareSymbols(t *testing.T) {
	reg := NewRegistry(&stubMD{
		ok: true,
		quote: marketdata.Quote{
			Symbol:       "BTC",
			PriceUsdt:    100000,
			Change24hPct: 1.5,
		},
		klines: marketdata.KlineResponse{
			Symbol:   "BTC",
			Interval: "1d",
			Candles: []binance.Candle{
				{Time: 1, Open: 100, High: 110, Low: 90, Close: 105, Volume: 10},
				{Time: 2, Open: 105, High: 120, Low: 100, Close: 115, Volume: 20},
			},
		},
	})
	out, err := reg.Execute(context.Background(), "compare_symbols", `{"symbols":["BTC","ETH"]}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if m["ok"] != true {
		t.Fatalf("%s", out)
	}
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items=%d", len(items))
	}
}

func TestCompactBoardBriefing(t *testing.T) {
	reg := NewRegistry(&stubMD{
		ok:    true,
		quote: marketdata.Quote{Symbol: "BTC", Change24hPct: 3, PriceUsdt: 1},
	})
	text := reg.CompactBoardBriefing()
	if text == "" || !strings.Contains(text, "看板实时盘面摘要") {
		t.Fatalf("briefing=%q", text)
	}
}

func TestMarketBreadth(t *testing.T) {
	reg := NewRegistry(&stubMD{
		center: marketcenter.CenterResponse{
			ChgDiagram: marketcenter.ChgDiagram{Up: 10, Down: 5, Balance: 2},
		},
	})
	out, err := reg.Execute(context.Background(), "get_market_breadth", `{"market":"cn"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if m["market"] != "cn" {
		t.Fatalf("%v", m["market"])
	}
	breadth := m["breadth"].(map[string]any)
	if breadth["up"].(float64) != 10 {
		t.Fatalf("up=%v", breadth["up"])
	}
}

func TestSymbolBrief(t *testing.T) {
	reg := NewRegistry(&stubMD{
		ok: true,
		quote: marketdata.Quote{
			Symbol:       "BTC",
			PriceUsdt:    100000,
			Change24hPct: 1.5,
		},
		klines: marketdata.KlineResponse{
			Symbol:   "BTC",
			Interval: "1d",
			Source:   "stub",
			Candles: []binance.Candle{
				{Time: 1, Open: 100, High: 110, Low: 90, Close: 105},
				{Time: 2, Open: 105, High: 120, Low: 100, Close: 115},
			},
		},
	})
	out, err := reg.Execute(context.Background(), "get_symbol_brief", `{"symbol":"BTC"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != true {
		t.Fatalf("%s", out)
	}
	quote, _ := m["quote"].(map[string]any)
	if quote["ok"] != true {
		t.Fatalf("quote=%v", quote)
	}
	kline, _ := m["kline1d"].(map[string]any)
	if kline["ok"] != true {
		t.Fatalf("kline=%v", kline)
	}
	if _, ok := kline["nearRangeHigh"]; !ok {
		t.Fatal("missing nearRangeHigh")
	}
}

func TestSnapshotSummary(t *testing.T) {
	reg := NewRegistry(&stubMD{
		ok:    true,
		quote: marketdata.Quote{Symbol: "BTC", Change24hPct: 3, PriceUsdt: 1},
	})
	out, err := reg.Execute(context.Background(), "get_snapshot_summary", `{"limit":1}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(out), &m)
	if m["ok"] != true {
		t.Fatal(out)
	}
}
