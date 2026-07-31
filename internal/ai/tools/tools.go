package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

// Registry exposes OpenAI-compatible tool definitions and executors.
type Registry struct {
	md marketdata.MarketDataService
}

func NewRegistry(md marketdata.MarketDataService) *Registry {
	return &Registry{md: md}
}

// Definition is one tool schema for the LLM.
type Definition struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

type FunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Definitions returns all Phase-1 tools.
func (r *Registry) Definitions() []Definition {
	return []Definition{
		{
			Type: "function",
			Function: FunctionSchema{
				Name: "get_symbol_brief",
				Description: "单标综合简报（优先用于「XX怎么样/怎么看」）：一次返回最新报价、近30日日K摘要、以及最多3条相关快讯。加密默认搜币圈快讯。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":     map[string]any{"type": "string", "description": "如 BTC、ETH、aapl、sh000001"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "alpha"}, "description": "可省略，按符号猜测"},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_quote",
				Description: "仅取单标最新报价（加密现货/指数/美股参考）。只要价格涨跌、不要走势时用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":     map[string]any{"type": "string", "description": "e.g. BTC, BTCUSDT, aapl, sh000001"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "alpha"}},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_snapshot_summary",
				Description: "币圈盘面摘要：汇率、总市值、恐贪情绪、BTC占比、涨跌幅 TopN。问「大盘/整体/情绪/涨跌榜」时用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{"type": "integer", "description": "涨跌榜条数，默认5，最大20"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_klines_summary",
				Description: "K线区间摘要：开高低收、区间涨跌%、最近一根涨跌、是否贴近区间高低、SMA7/SMA30、收盘相对均线位置、最近成交量相对均值。不含原始逐根序列。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":   map[string]any{"type": "string"},
						"interval": map[string]any{"type": "string", "description": "如 15m,1h,4h,1d，默认1d"},
						"limit":    map[string]any{"type": "integer", "description": "根数，默认30，最大100"},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "compare_symbols",
				Description: "批量对比 2–5 个标的：各自最新报价 + 日K关键指标（区间涨跌、均线位置、量能）。问「BTC和ETH比/谁更强/相对强弱」时优先用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbols": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "如 [\"BTC\",\"ETH\"]",
							"minItems":    2,
							"maxItems":    5,
						},
					},
					"required": []string{"symbols"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_express_news",
				Description: "财经/币圈快讯标题列表。问新闻、快讯、发生了什么时用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tag":   map[string]any{"type": "string", "description": "如 币圈、A股"},
						"limit": map[string]any{"type": "integer", "description": "默认8，最大20"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_market_breadth",
				Description: "股市涨跌家数与热门板块（A股 cn / 港股 hk / 美股 us）。问「涨跌家数、市场宽度、热门板块」时用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"market": map[string]any{"type": "string", "enum": []string{"cn", "hk", "us"}},
					},
					"required": []string{"market"},
				},
			},
		},
	}
}

// Execute runs a named tool and returns compact JSON text.
func (r *Registry) Execute(ctx context.Context, name, argumentsJSON string) (string, error) {
	var args map[string]any
	if strings.TrimSpace(argumentsJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case "get_symbol_brief":
		return r.getSymbolBrief(ctx, args)
	case "get_quote":
		return r.getQuote(args)
	case "get_snapshot_summary":
		return r.getSnapshotSummary(args)
	case "get_klines_summary":
		return r.getKlinesSummary(args)
	case "compare_symbols":
		return r.compareSymbols(ctx, args)
	case "get_express_news":
		return r.getExpressNews(ctx, args)
	case "get_market_breadth":
		return r.getMarketBreadth(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func asInt(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return def
	}
}

func normalizeCryptoSymbol(sym string) string {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	sym = strings.TrimSuffix(sym, "USDT")
	sym = strings.TrimSuffix(sym, "USD")
	return sym
}

func (r *Registry) getQuote(args map[string]any) (string, error) {
	symbol := asString(args["symbol"])
	if symbol == "" {
		return marshal(map[string]any{"ok": false, "error": "symbol required"})
	}
	asset := strings.ToLower(asString(args["assetClass"]))
	if asset == "" {
		asset = guessAssetClass(symbol)
	}
	switch asset {
	case "index":
		q, ok := r.md.IndexQuote(strings.ToLower(symbol))
		if !ok {
			return marshal(map[string]any{"ok": false, "error": "index quote not found", "symbol": symbol})
		}
		return marshal(map[string]any{
			"ok": true, "assetClass": "index", "id": q.ID, "name": q.Name,
			"price": q.Price, "changePct": q.ChangePct, "updatedAt": q.UpdatedAt,
		})
	case "alpha":
		id := strings.ToLower(symbol)
		id = strings.TrimSuffix(id, "usdt")
		q, ok := r.md.AlphaQuote(id)
		if !ok {
			return marshal(map[string]any{"ok": false, "error": "alpha quote not found", "symbol": symbol})
		}
		return marshal(map[string]any{
			"ok": true, "assetClass": "alpha", "id": q.ID, "name": q.Name, "symbol": q.Symbol,
			"price": q.Price, "changeDayPct": q.ChangeDayPct, "change24hPct": q.Change24hPct, "updatedAt": q.UpdatedAt,
		})
	default:
		base := normalizeCryptoSymbol(symbol)
		q, ok := r.md.Quote(base)
		if !ok {
			return marshal(map[string]any{"ok": false, "error": "crypto quote not found", "symbol": base})
		}
		return marshal(map[string]any{
			"ok": true, "assetClass": "crypto", "symbol": q.Symbol,
			"priceUsdt": q.PriceUsdt, "priceCny": q.PriceCny,
			"change24hPct": q.Change24hPct, "changeDayPct": q.ChangeDayPct, "updatedAt": q.UpdatedAt,
		})
	}
}

func guessAssetClass(symbol string) string {
	s := strings.ToLower(symbol)
	if strings.HasPrefix(s, "sh") || strings.HasPrefix(s, "sz") || strings.Contains(s, "000001") {
		return "index"
	}
	switch s {
	case "hsi", "nikkei", "dji", "ixic", "spx", "ftse", "gdaui", "ndx":
		return "index"
	}
	return "crypto"
}

func (r *Registry) getSnapshotSummary(args map[string]any) (string, error) {
	limit := asInt(args["limit"], 5)
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	snap := r.md.Snapshot()
	type mover struct {
		Symbol string  `json:"symbol"`
		Pct    float64 `json:"change24hPct"`
		Price  float64 `json:"priceUsdt"`
	}
	quotes := append([]marketdata.Quote(nil), snap.Quotes...)
	sort.Slice(quotes, func(i, j int) bool { return quotes[i].Change24hPct > quotes[j].Change24hPct })
	gainers := make([]mover, 0, limit)
	for i := 0; i < len(quotes) && i < limit; i++ {
		gainers = append(gainers, mover{quotes[i].Symbol, quotes[i].Change24hPct, quotes[i].PriceUsdt})
	}
	sort.Slice(quotes, func(i, j int) bool { return quotes[i].Change24hPct < quotes[j].Change24hPct })
	losers := make([]mover, 0, limit)
	for i := 0; i < len(quotes) && i < limit; i++ {
		losers = append(losers, mover{quotes[i].Symbol, quotes[i].Change24hPct, quotes[i].PriceUsdt})
	}
	return marshal(map[string]any{
		"ok":      true,
		"ts":      snap.Ts,
		"version": snap.Version,
		"rates": map[string]any{
			"usdtCny": snap.Rates.USDTCNY,
			"usdCny":  snap.Rates.USDCNY,
		},
		"macro": map[string]any{
			"totalMarketCapUsd": snap.Macro.TotalMarketCapUsd,
			"fearGreed":         snap.Macro.FearGreed,
			"btcDominancePct":   snap.Macro.BTCDominancePct,
		},
		"topGainers": gainers,
		"topLosers":  losers,
		"quoteCount": len(snap.Quotes),
		"indexCount": len(snap.Indices),
	})
}

func (r *Registry) getKlinesSummary(args map[string]any) (string, error) {
	symbol := asString(args["symbol"])
	if symbol == "" {
		return marshal(map[string]any{"ok": false, "error": "symbol required"})
	}
	interval := asString(args["interval"])
	if interval == "" {
		interval = "1d"
	}
	limit := asInt(args["limit"], 30)
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	base := normalizeCryptoSymbol(symbol)
	resp, err := r.md.Klines(base, interval, limit)
	if err != nil {
		// try index
		idxResp, idxErr := r.md.IndexKlines(strings.ToLower(symbol), interval, limit)
		if idxErr != nil {
			return marshal(map[string]any{"ok": false, "error": err.Error(), "symbol": symbol})
		}
		resp = idxResp
	}
	if len(resp.Candles) == 0 {
		return marshal(map[string]any{"ok": false, "error": "no candles", "symbol": resp.Symbol})
	}
	first := resp.Candles[0]
	last := resp.Candles[len(resp.Candles)-1]
	high, low := last.High, last.Low
	for _, c := range resp.Candles {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	chg := 0.0
	if first.Open > 0 {
		chg = (last.Close - first.Open) / first.Open * 100
	}
	lastBarChg := 0.0
	if last.Open > 0 {
		lastBarChg = (last.Close - last.Open) / last.Open * 100
	}
	nearHigh := false
	nearLow := false
	rangePos := 0.0
	if high > low {
		rangePos = (last.Close - low) / (high - low)
		nearHigh = rangePos >= 0.85
		nearLow = rangePos <= 0.15
	}

	sma7 := smaClose(resp.Candles, 7)
	sma30 := smaClose(resp.Candles, 30)
	vsSMA7 := ""
	vsSMA30 := ""
	if sma7 > 0 {
		if last.Close >= sma7 {
			vsSMA7 = "above"
		} else {
			vsSMA7 = "below"
		}
	}
	if sma30 > 0 {
		if last.Close >= sma30 {
			vsSMA30 = "above"
		} else {
			vsSMA30 = "below"
		}
	}

	lastVol := last.Volume
	avgVol := avgVolume(resp.Candles, len(resp.Candles)-1) // exclude last bar when possible
	volRatio := 0.0
	if avgVol > 0 {
		volRatio = lastVol / avgVol
	}
	volNote := "unknown"
	if avgVol > 0 {
		switch {
		case volRatio >= 1.5:
			volNote = "high"
		case volRatio <= 0.7:
			volNote = "low"
		default:
			volNote = "normal"
		}
	}

	out := map[string]any{
		"ok": true, "symbol": resp.Symbol, "interval": resp.Interval, "source": resp.Source,
		"candleCount":      len(resp.Candles),
		"open":             first.Open,
		"close":            last.Close,
		"high":             high,
		"low":              low,
		"rangeChangePct":   round2(chg),
		"lastBarChangePct": round2(lastBarChg),
		"nearRangeHigh":    nearHigh,
		"nearRangeLow":     nearLow,
		"rangePosition":    round2(rangePos),
		"firstOpenTime":    first.Time,
		"lastOpenTime":     last.Time,
	}
	if sma7 > 0 {
		out["sma7"] = round2(sma7)
		out["closeVsSma7"] = vsSMA7
	}
	if sma30 > 0 {
		out["sma30"] = round2(sma30)
		out["closeVsSma30"] = vsSMA30
	}
	if avgVol > 0 || lastVol > 0 {
		out["lastVolume"] = round2(lastVol)
		out["avgVolume"] = round2(avgVol)
		out["volumeRatio"] = round2(volRatio)
		out["volumeNote"] = volNote
	}
	return marshal(out)
}

func smaClose(candles []binance.Candle, period int) float64 {
	n := len(candles)
	if period <= 0 || n < period {
		return 0
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += candles[i].Close
	}
	return sum / float64(period)
}

func avgVolume(candles []binance.Candle, endExclusive int) float64 {
	if endExclusive <= 0 {
		endExclusive = len(candles)
	}
	if endExclusive > len(candles) {
		endExclusive = len(candles)
	}
	if endExclusive <= 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < endExclusive; i++ {
		sum += candles[i].Volume
	}
	return sum / float64(endExclusive)
}

func (r *Registry) getSymbolBrief(ctx context.Context, args map[string]any) (string, error) {
	symbol := asString(args["symbol"])
	if symbol == "" {
		return marshal(map[string]any{"ok": false, "error": "symbol required"})
	}
	asset := strings.ToLower(asString(args["assetClass"]))
	if asset == "" {
		asset = guessAssetClass(symbol)
	}

	quoteRaw, err := r.getQuote(args)
	if err != nil {
		return "", err
	}
	var quoteObj map[string]any
	_ = json.Unmarshal([]byte(quoteRaw), &quoteObj)

	klineArgs := map[string]any{"symbol": symbol, "interval": "1d", "limit": 30}
	if asset != "" {
		klineArgs["assetClass"] = asset
	}
	klineRaw, err := r.getKlinesSummary(klineArgs)
	if err != nil {
		klineRaw = `{"ok":false,"error":"klines unavailable"}`
	}
	var klineObj map[string]any
	_ = json.Unmarshal([]byte(klineRaw), &klineObj)

	newsTag := "币圈"
	if asset == "index" {
		newsTag = "A股"
	}
	newsRaw, err := r.getExpressNews(ctx, map[string]any{"tag": newsTag, "limit": 3})
	if err != nil {
		newsRaw = `{"ok":false,"error":"news unavailable"}`
	}
	var newsObj map[string]any
	_ = json.Unmarshal([]byte(newsRaw), &newsObj)
	if items, ok := newsObj["items"].([]any); ok && len(items) > 3 {
		newsObj["items"] = items[:3]
	}

	return marshal(map[string]any{
		"ok":         true,
		"symbol":     symbol,
		"assetClass": asset,
		"quote":      quoteObj,
		"kline1d":    klineObj,
		"news":       newsObj,
	})
}

func (r *Registry) compareSymbols(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	syms := asStringSlice(args["symbols"])
	if len(syms) < 2 {
		return marshal(map[string]any{"ok": false, "error": "symbols requires 2–5 items"})
	}
	if len(syms) > 5 {
		syms = syms[:5]
	}
	items := make([]map[string]any, 0, len(syms))
	for _, sym := range syms {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		asset := guessAssetClass(sym)
		quoteRaw, _ := r.getQuote(map[string]any{"symbol": sym, "assetClass": asset})
		var quoteObj map[string]any
		_ = json.Unmarshal([]byte(quoteRaw), &quoteObj)

		klineRaw, _ := r.getKlinesSummary(map[string]any{"symbol": sym, "interval": "1d", "limit": 30})
		var klineObj map[string]any
		_ = json.Unmarshal([]byte(klineRaw), &klineObj)

		row := map[string]any{
			"symbol":     sym,
			"assetClass": asset,
			"quote":      quoteObj,
			"kline1d":    klineObj,
		}
		if quoteObj["ok"] == true {
			switch asset {
			case "crypto":
				row["change24hPct"] = quoteObj["change24hPct"]
				row["price"] = quoteObj["priceUsdt"]
			case "index":
				row["changePct"] = quoteObj["changePct"]
				row["price"] = quoteObj["price"]
			case "alpha":
				row["changeDayPct"] = quoteObj["changeDayPct"]
				row["price"] = quoteObj["price"]
			}
		}
		if klineObj["ok"] == true {
			row["rangeChangePct"] = klineObj["rangeChangePct"]
			row["closeVsSma7"] = klineObj["closeVsSma7"]
			row["closeVsSma30"] = klineObj["closeVsSma30"]
			row["volumeNote"] = klineObj["volumeNote"]
			row["nearRangeHigh"] = klineObj["nearRangeHigh"]
			row["nearRangeLow"] = klineObj["nearRangeLow"]
		}
		items = append(items, row)
	}
	if len(items) < 2 {
		return marshal(map[string]any{"ok": false, "error": "need at least 2 valid symbols"})
	}
	return marshal(map[string]any{"ok": true, "count": len(items), "items": items})
}

// CompactBoardBriefing builds a short live board text for page_context (server-side).
func (r *Registry) CompactBoardBriefing() string {
	if r == nil || r.md == nil {
		return ""
	}
	raw, err := r.getSnapshotSummary(map[string]any{"limit": 3})
	if err != nil {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil || m["ok"] != true {
		return ""
	}
	var b strings.Builder
	b.WriteString("看板实时盘面摘要:\n")
	if rates, ok := m["rates"].(map[string]any); ok {
		fmt.Fprintf(&b, "- 汇率 U/CNY=%v USD/CNY=%v\n", rates["usdtCny"], rates["usdCny"])
	}
	if macro, ok := m["macro"].(map[string]any); ok {
		fg, _ := macro["fearGreed"].(map[string]any)
		if fg == nil {
			// FearGreed might be struct encoded as map with value/label
			fmt.Fprintf(&b, "- 宏观 市值=%v BTC占比=%v\n", macro["totalMarketCapUsd"], macro["btcDominancePct"])
		} else {
			fmt.Fprintf(&b, "- 情绪=%v %v · BTC占比=%v · 总市值=%v\n",
				fg["value"], fg["label"], macro["btcDominancePct"], macro["totalMarketCapUsd"])
		}
	}
	if gainers, ok := m["topGainers"].([]any); ok && len(gainers) > 0 {
		b.WriteString("- 涨幅前: ")
		for i, g := range gainers {
			if i > 0 {
				b.WriteString(", ")
			}
			if row, ok := g.(map[string]any); ok {
				fmt.Fprintf(&b, "%v %+v%%", row["symbol"], row["change24hPct"])
			}
		}
		b.WriteByte('\n')
	}
	if losers, ok := m["topLosers"].([]any); ok && len(losers) > 0 {
		b.WriteString("- 跌幅前: ")
		for i, g := range losers {
			if i > 0 {
				b.WriteString(", ")
			}
			if row, ok := g.(map[string]any); ok {
				fmt.Fprintf(&b, "%v %+v%%", row["symbol"], row["change24hPct"])
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s := strings.TrimSpace(fmt.Sprint(x))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}

func (r *Registry) getExpressNews(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	tag := asString(args["tag"])
	limit := asInt(args["limit"], 8)
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	resp, err := r.md.ExpressNews(tag, 1, limit, 0)
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error()})
	}
	items := make([]map[string]any, 0, len(resp.Items))
	for _, it := range resp.Items {
		body := it.Body
		if len([]rune(body)) > 120 {
			body = string([]rune(body)[:120]) + "…"
		}
		items = append(items, map[string]any{
			"title": it.Title, "body": body, "publishTime": it.PublishTime,
			"tag": it.Tag, "important": it.Important,
		})
	}
	return marshal(map[string]any{
		"ok": true, "tag": resp.Tag, "source": resp.Source, "fetchedAt": resp.FetchedAt, "items": items,
	})
}

func (r *Registry) getMarketBreadth(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	market := strings.ToLower(asString(args["market"]))
	if market == "" {
		market = "cn"
	}
	resp, err := r.md.MarketCenter(market)
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error(), "market": market})
	}
	hot := make([]map[string]any, 0, 5)
	if len(resp.Overview.Tabs) > 0 {
		for i, item := range resp.Overview.Tabs[0].Items {
			if i >= 5 {
				break
			}
			hot = append(hot, map[string]any{
				"name": item.Name, "changePct": item.ChangePct, "leadName": item.LeadName,
			})
		}
	}
	return marshal(map[string]any{
		"ok": true, "market": resp.Market, "marketActive": resp.MarketActive, "source": resp.Source,
		"breadth": map[string]any{
			"up": resp.ChgDiagram.Up, "down": resp.ChgDiagram.Down, "balance": resp.ChgDiagram.Balance,
			"totalTitle": resp.ChgDiagram.TotalTitle, "totalValue": resp.ChgDiagram.TotalValue,
		},
		"hotSectors": hot,
		"fetchedAt":  resp.FetchedAt,
	})
}

func marshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Now is exported for tests if needed.
var Now = time.Now
