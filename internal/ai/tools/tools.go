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
	md  marketdata.MarketDataService
	now func() time.Time
}

func NewRegistry(md marketdata.MarketDataService) *Registry {
	return &Registry{md: md, now: time.Now}
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
				Name:        "get_symbol_brief",
				Description: "单标综合简报（优先用于「XX怎么样/怎么看」）：一次返回最新报价、近30日日K摘要、以及最多3条相关快讯。加密默认搜币圈快讯。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":     map[string]any{"type": "string", "description": "如 BTC、ETH、aapl、sh000001"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "us_equity", "us_contract", "alpha"}, "description": "裸美股 ticker 默认现货 us_equity；合约参考显式 us_contract；alpha 为兼容输入"},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_quote",
				Description: "单标报价。美股默认现货 USD / 交易日涨跌；合约参考必须指定 us_contract，返回 USDT / 滚动24h及实际工具类型。不可相互替代。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":     map[string]any{"type": "string", "description": "e.g. BTC, BTCUSDT, aapl, sh000001"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "us_equity", "us_contract", "alpha"}},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_snapshot_summary",
				Description: "币圈盘面摘要：汇率、总市值、恐贪情绪、BTC占比、涨跌幅 TopN。问「大盘/整体/情绪/涨跌榜」时用。更细的资金费率/爆仓/多空等用 get_macro_metrics。",
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
				Name:        "get_macro_metrics",
				Description: "币圈宏观指标全集（与首页 MacroGrid 对齐）：总市值/成交额、恐贪、BTC/ETH 占比、稳定币市值、资金费率与溢价、持仓量、爆仓、多空比、大户多空、主动买卖比。问资金费率/爆仓/多空/稳定币时用。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_index_board",
				Description: "全球指数看板列表（与首页指数区对齐）：各指数现价与涨跌幅。问「指数怎么样/纳指道指沪深」时用。",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_alpha_board",
				Description: "仅美股合约/代币化参考工具看板，非股票现货；保留旧工具名兼容。现货单标请用 get_quote + us_equity。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{"type": "string", "description": "all|index|stock，默认 all"},
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
						"symbol":     map[string]any{"type": "string"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "us_equity", "us_contract", "alpha"}},
						"interval":   map[string]any{"type": "string", "description": "如 15m,1h,4h,1d，默认1d"},
						"limit":      map[string]any{"type": "integer", "description": "根数，默认30，最大100"},
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
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "us_equity", "us_contract", "alpha"}, "description": "所有对比标的使用同一口径；不传则各自按标的推断"},
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
						"tag":        map[string]any{"type": "string", "description": "如 币圈、A股"},
						"symbol":     map[string]any{"type": "string", "description": "可选，按标的实体/名称/ticker匹配新闻，不足不以其他标的新闻补齐"},
						"assetClass": map[string]any{"type": "string", "enum": []string{"crypto", "index", "us_equity", "us_contract", "alpha"}},
						"limit":      map[string]any{"type": "integer", "description": "默认8，最大20"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_market_breadth",
				Description: "股市行情中心（与首页行情中心对齐）：涨跌家数、涨跌分布 bars、热门板块、热力图 Top、主力资金流 Top。market：A股 ab（可用 cn）、港股 hk、美股 us。问涨跌家数/热门板块/资金流/热力图时用；休市时仍可能返回上一交易时段数据。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"market": map[string]any{
							"type":        "string",
							"description": "ab/cn=A股，hk=港股，us/usa/美股=美股",
						},
					},
					"required": []string{"market"},
				},
			},
		},
	}
}

// Execute runs a named tool and returns compact JSON text.
func (r *Registry) Execute(ctx context.Context, name, argumentsJSON string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if r == nil || r.md == nil {
		return "", fmt.Errorf("market data unavailable")
	}
	var args map[string]any
	if strings.TrimSpace(argumentsJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}
	raw, err := r.execute(ctx, name, args)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	return decorateResult(name, raw, r.now())
}

func (r *Registry) execute(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "get_symbol_brief":
		return r.getSymbolBrief(ctx, args)
	case "get_quote":
		return r.getQuote(args)
	case "get_snapshot_summary":
		return r.getSnapshotSummary(args)
	case "get_klines_summary":
		return r.getKlinesSummary(ctx, args)
	case "compare_symbols":
		return r.compareSymbols(ctx, args)
	case "get_express_news":
		return r.getExpressNews(ctx, args)
	case "get_market_breadth":
		return r.getMarketBreadth(ctx, args)
	case "get_macro_metrics":
		return r.getMacroMetrics(args)
	case "get_index_board":
		return r.getIndexBoard(args)
	case "get_alpha_board":
		return r.getAlphaBoard(args)
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
		"source":  "marketdata.snapshot",
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

func (r *Registry) getKlinesSummary(ctx context.Context, args map[string]any) (string, error) {
	symbol := asString(args["symbol"])
	inst, resolveErr := r.resolveInstrument(symbol, asString(args["assetClass"]))
	if resolveErr != nil {
		return marshal(map[string]any{"ok": false, "error": resolveErr.Error()})
	}
	interval := asString(args["interval"])
	if interval == "" {
		interval = "1d"
	}
	interval, intervalErr := binance.NormalizeInterval(interval)
	if intervalErr != nil {
		return marshal(map[string]any{"ok": false, "error": intervalErr.Error()})
	}
	limit := asInt(args["limit"], 30)
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	var resp marketdata.KlineResponse
	var err error
	if inst.Asset == "index" || inst.Asset == "us_equity" {
		resp, err = r.md.IndexKlinesContext(ctx, inst.ID, interval, limit)
	} else {
		base := inst.Symbol
		if inst.Asset == "us_contract" {
			if q, ok := r.md.AlphaQuote(inst.ID); ok && q.Symbol != "" {
				base = normalizeCryptoSymbol(q.Symbol)
			}
		}
		resp, err = r.md.KlinesContext(ctx, base, interval, limit)
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error(), "symbol": symbol, "assetClass": inst.Asset})
	}
	resolvedResponse, responseErr := r.resolveInstrument(resp.Symbol, inst.Asset)
	if responseErr != nil || resolvedResponse.ID != inst.ID || !strings.EqualFold(resp.Interval, interval) {
		return marshal(map[string]any{"ok": false, "error": "kline response does not match the requested instrument", "symbol": symbol})
	}
	refSource := resp.Source == "bitget" || resp.Source == "binance-alpha"
	if inst.Asset != "us_contract" && refSource || inst.Asset == "us_contract" && resp.Source == "binance" {
		return marshal(map[string]any{"ok": false, "error": "kline source belongs to a different instrument type", "symbol": symbol})
	}
	if len(resp.Candles) == 0 {
		return marshal(map[string]any{"ok": false, "error": "no candles", "symbol": resp.Symbol})
	}
	first := resp.Candles[0]
	last := resp.Candles[len(resp.Candles)-1]
	high, low := last.High, last.Low
	for _, c := range resp.Candles {
		for _, price := range []float64{c.Open, c.High, c.Low, c.Close} {
			if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
				return marshal(map[string]any{"ok": false, "error": "invalid candle OHLC values", "symbol": symbol})
			}
		}
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

	out := inst.fields(resp.Source)
	out["changeBasis"] = "candle_range"
	for key, value := range map[string]any{
		"ok": true, "interval": resp.Interval, "source": resp.Source, "pair": resp.Pair,
		"fetchedAt": r.now(), "lastCandleClosed": last.Closed,
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
	} {
		out[key] = value
	}
	if inst.Asset == "us_contract" {
		out["limitations"] = []string{"历史参考工具须按实际 source / instrumentType 解读；不可当作股票现货"}
	}
	if !last.Closed {
		out["limitations"] = append(asStringSlice(out["limitations"]), "最新K线未确认收盘，价格和量能仍可能变化")
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
	inst, err := r.resolveInstrument(asString(args["symbol"]), asString(args["assetClass"]))
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error()})
	}
	out := r.symbolData(ctx, args, inst)
	newsRaw, newsErr := r.getExpressNews(ctx, map[string]any{
		"symbol": asString(args["symbol"]), "assetClass": inst.Asset, "limit": 3,
	})
	out["news"] = resultObject(newsRaw, newsErr)
	return marshal(out)
}

func resultObject(raw string, err error) map[string]any {
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	var out map[string]any
	if json.Unmarshal([]byte(raw), &out) != nil || out == nil {
		return map[string]any{"ok": false, "error": "invalid tool result"}
	}
	return out
}

func (r *Registry) symbolData(ctx context.Context, args map[string]any, inst instrument) map[string]any {
	quoteRaw, quoteErr := r.getQuote(args)
	quote := resultObject(quoteRaw, quoteErr)
	klineRaw, klineErr := r.getKlinesSummary(ctx, map[string]any{
		"symbol": asString(args["symbol"]), "assetClass": inst.Asset, "interval": "1d", "limit": 30,
	})
	kline := resultObject(klineRaw, klineErr)
	out := map[string]any{"ok": true, "symbol": inst.Symbol, "assetClass": inst.Asset, "quote": quote, "kline1d": kline}
	if quote["instrumentType"] != nil && kline["instrumentType"] != nil && quote["instrumentType"] != kline["instrumentType"] {
		out["partial"] = true
		out["limitations"] = []string{"报价与历史K线属于不同参考工具，不得直接比较价格、均线或合并涨跌幅"}
	}
	return out
}

func (r *Registry) compareSymbols(ctx context.Context, args map[string]any) (string, error) {
	syms := asStringSlice(args["symbols"])
	if len(syms) < 2 || len(syms) > 5 {
		return marshal(map[string]any{"ok": false, "error": "symbols requires 2-5 items"})
	}
	items := make([]map[string]any, 0, len(syms))
	bases, currencies := map[string]bool{}, map[string]bool{}
	for _, sym := range syms {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		inst, err := r.resolveInstrument(sym, asString(args["assetClass"]))
		if err != nil {
			items = append(items, map[string]any{"ok": false, "symbol": sym, "error": err.Error()})
			continue
		}
		row := r.symbolData(ctx, map[string]any{"symbol": sym, "assetClass": inst.Asset}, inst)
		quote := row["quote"].(map[string]any)
		if quote["ok"] == true {
			bases[asString(quote["changeBasis"])], currencies[asString(quote["currency"])] = true, true
			row["price"] = quote["price"]
			if inst.Asset == "crypto" {
				row["price"] = quote["priceUsdt"]
			}
			row["changeBasis"] = quote["changeBasis"]
			row["currency"] = quote["currency"]
			row["changePct"], row["change24hPct"] = quote["changePct"], quote["change24hPct"]
		}
		items = append(items, row)
	}
	out := map[string]any{"ok": true, "count": len(items), "items": items}
	if len(bases) > 1 || len(currencies) > 1 {
		out["partial"] = true
		out["limitations"] = []string{"报价货币或涨跌周期不同，仅逐项展示；不得直接按这些涨跌幅排名，也不得假设 USD 与 USDT 等值"}
	}
	return marshal(out)
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

func (r *Registry) getMarketBreadth(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	market := normalizeBreadthMarket(asString(args["market"]))
	resp, err := r.md.MarketCenter(market)
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error(), "market": market})
	}

	hot := make([]map[string]any, 0, 8)
	if len(resp.Overview.Tabs) > 0 {
		for i, item := range resp.Overview.Tabs[0].Items {
			if i >= 8 {
				break
			}
			hot = append(hot, map[string]any{
				"name": item.Name, "changePct": item.ChangePct, "leadName": item.LeadName,
				"leadChangePct": item.LeadChangePct, "price": item.Price,
			})
		}
	}

	heatmap := make([]map[string]any, 0, 12)
	for i, item := range resp.Heatmap.Items {
		if i >= 12 {
			break
		}
		heatmap = append(heatmap, map[string]any{
			"code": item.Code, "name": item.Name, "changePct": item.PxChangeRate,
			"amount": item.Amount, "marketValue": item.MarketValue,
		})
	}
	if len(hot) == 0 {
		for i, item := range heatmap {
			if i >= 5 {
				break
			}
			hot = append(hot, map[string]any{
				"name": item["name"], "changePct": item["changePct"], "code": item["code"],
			})
		}
	}

	bars := make([]map[string]any, 0, len(resp.ChgDiagram.Bars))
	for _, b := range resp.ChgDiagram.Bars {
		bars = append(bars, map[string]any{"title": b.Title, "status": b.Status, "count": b.Count})
	}

	fundflow := make([]map[string]any, 0, 3)
	for _, g := range resp.Fundflow.Groups {
		items := make([]map[string]any, 0, 5)
		for i, it := range g.Items {
			if i >= 5 {
				break
			}
			items = append(items, map[string]any{
				"code": it.Code, "name": it.Name,
				"netAmount": it.NetAmount, "mainNetTurnover": it.MainNetTurnover,
			})
		}
		if len(items) == 0 {
			continue
		}
		fundflow = append(fundflow, map[string]any{
			"blockType": g.BlockType, "blockTypeName": g.BlockTypeName, "items": items,
		})
	}

	breadthTotal := resp.ChgDiagram.Up + resp.ChgDiagram.Down + resp.ChgDiagram.Balance
	if breadthTotal == 0 && len(hot) == 0 && len(heatmap) == 0 && len(fundflow) == 0 {
		return marshal(map[string]any{
			"ok":           false,
			"error":        "empty market breadth from upstream",
			"market":       resp.Market,
			"marketActive": resp.MarketActive,
			"hint":         "上游暂无涨跌家数/板块；可改问指数或个股报价",
		})
	}

	sessionNote := "latest_available"
	if !resp.MarketActive {
		sessionNote = "outside_session_showing_last_available"
	}
	return marshal(map[string]any{
		"ok":           true,
		"market":       resp.Market,
		"marketActive": resp.MarketActive,
		"sessionNote":  sessionNote,
		"source":       resp.Source,
		"breadth": map[string]any{
			"up": resp.ChgDiagram.Up, "down": resp.ChgDiagram.Down, "balance": resp.ChgDiagram.Balance,
			"totalTitle": resp.ChgDiagram.TotalTitle, "totalValue": resp.ChgDiagram.TotalValue,
			"bars": bars,
		},
		"hotSectors": hot,
		"heatmapTop": heatmap,
		"fundflow":   fundflow,
		"fetchedAt":  resp.FetchedAt,
		"hint":       "若 marketActive=false，数据仍是最近可得盘面，请说明为上一交易时段而非编造「暂无数据」",
	})
}

func (r *Registry) getMacroMetrics(args map[string]any) (string, error) {
	_ = args
	snap := r.md.Snapshot()
	m := snap.Macro
	return marshal(map[string]any{
		"ok":     true,
		"source": "marketdata.snapshot",
		"ts":     snap.Ts,
		"rates": map[string]any{
			"usdtCny":   snap.Rates.USDTCNY,
			"usdCny":    snap.Rates.USDCNY,
			"updatedAt": snap.Rates.UpdatedAt,
		},
		"macro": map[string]any{
			"totalMarketCapUsd":               m.TotalMarketCapUsd,
			"totalVolume24hUsd":               m.TotalVolume24hUsd,
			"totalMarketCapChange24hPct":      m.TotalMarketCapChange24hPct,
			"fearGreed":                       m.FearGreed,
			"btcDominancePct":                 m.BTCDominancePct,
			"ethDominancePct":                 m.ETHDominancePct,
			"stablecoinMarketCapUsd":          m.StablecoinMarketCapUsd,
			"stablecoinMarketCapChange24hPct": m.StablecoinMarketCapChange24hPct,
			"funding": map[string]any{
				"symbol": m.Funding.Symbol, "rate": m.Funding.Rate,
				"premiumPct": m.Funding.PremiumPct, "markPrice": m.Funding.MarkPrice,
				"indexPrice": m.Funding.IndexPrice, "nextFundingTime": m.Funding.NextFundingTime,
				"updatedAt": m.Funding.UpdatedAt,
			},
			"openInterest": map[string]any{
				"symbol": m.OpenInterest.Symbol, "valueUsd": m.OpenInterest.ValueUsd,
				"changePct": m.OpenInterest.ChangePct, "updatedAt": m.OpenInterest.UpdatedAt,
			},
			"liquidations": map[string]any{
				"window": m.Liquidations.Window, "longUsd": m.Liquidations.LongUsd,
				"shortUsd": m.Liquidations.ShortUsd, "totalUsd": m.Liquidations.TotalUsd,
				"updatedAt": m.Liquidations.UpdatedAt,
			},
			"longShort": map[string]any{
				"symbol": m.LongShort.Symbol, "ratio": m.LongShort.Ratio,
				"longAccountPct": m.LongShort.LongAccountPct, "shortAccountPct": m.LongShort.ShortAccountPct,
				"updatedAt": m.LongShort.UpdatedAt,
			},
			"topLongShort": map[string]any{
				"symbol": m.TopLongShort.Symbol, "ratio": m.TopLongShort.Ratio,
				"longAccountPct": m.TopLongShort.LongAccountPct, "shortAccountPct": m.TopLongShort.ShortAccountPct,
				"updatedAt": m.TopLongShort.UpdatedAt,
			},
			"takerBuySell": map[string]any{
				"symbol": m.TakerBuySell.Symbol, "ratio": m.TakerBuySell.Ratio,
				"buyVol": m.TakerBuySell.BuyVol, "sellVol": m.TakerBuySell.SellVol,
				"updatedAt": m.TakerBuySell.UpdatedAt,
			},
		},
	})
}

func (r *Registry) getIndexBoard(args map[string]any) (string, error) {
	_ = args
	snap := r.md.Snapshot()
	items := make([]map[string]any, 0, len(snap.Indices))
	for _, q := range snap.Indices {
		raw, err := r.getQuote(map[string]any{"symbol": q.ID, "assetClass": "index"})
		items = append(items, resultObject(raw, err))
	}
	return marshal(map[string]any{
		"ok": len(items) > 0, "count": len(items), "items": items, "ts": snap.Ts,
	})
}

func (r *Registry) getAlphaBoard(args map[string]any) (string, error) {
	cat := strings.ToLower(asString(args["category"]))
	if cat == "" {
		cat = "all"
	}
	snap := r.md.Snapshot()
	pack := func(list []marketdata.AlphaQuote) []map[string]any {
		out := make([]map[string]any, 0, len(list))
		for _, q := range list {
			raw, err := r.getQuote(map[string]any{"symbol": q.ID, "assetClass": "us_contract"})
			out = append(out, resultObject(raw, err))
		}
		return out
	}
	out := map[string]any{
		"ok": true, "source": snap.Alpha.Source, "updatedAt": snap.Alpha.UpdatedAt, "ts": snap.Ts,
	}
	switch cat {
	case "index":
		out["indices"] = pack(snap.Alpha.Indices)
		out["count"] = len(snap.Alpha.Indices)
	case "stock":
		out["stocks"] = pack(snap.Alpha.Stocks)
		out["count"] = len(snap.Alpha.Stocks)
	default:
		out["indices"] = pack(snap.Alpha.Indices)
		out["stocks"] = pack(snap.Alpha.Stocks)
		out["count"] = len(snap.Alpha.Indices) + len(snap.Alpha.Stocks)
	}
	if out["count"] == 0 {
		out["ok"] = false
		out["error"] = "US reference board unavailable"
	}
	return marshal(out)
}

func normalizeBreadthMarket(raw string) string {
	m := strings.ToLower(strings.TrimSpace(raw))
	switch m {
	case "", "cn", "china", "a", "a股", "ashare", "沪深":
		return "ab"
	case "usa", "nyse", "nasdaq", "美股", "美国":
		return "us"
	case "港股", "hongkong", "hkex":
		return "hk"
	default:
		if m == "" {
			return "ab"
		}
		return m
	}
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
