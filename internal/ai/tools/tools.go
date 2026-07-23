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
				Name:        "get_quote",
				Description: "Get latest quote for a crypto spot symbol, index id, or alpha stock id",
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
				Description: "Summarize market snapshot: rates, macro fear/greed, top gainers/losers",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"limit": map[string]any{"type": "integer", "description": "top N movers, default 5"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_klines_summary",
				Description: "Summarize recent OHLCV candles (range change, high/low) without dumping raw series",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"symbol":   map[string]any{"type": "string"},
						"interval": map[string]any{"type": "string", "description": "e.g. 15m,1h,4h,1d"},
						"limit":    map[string]any{"type": "integer"},
					},
					"required": []string{"symbol"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_express_news",
				Description: "Fetch recent finance/crypto flash news headlines",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tag":   map[string]any{"type": "string", "description": "optional tag e.g. 币圈, A股"},
						"limit": map[string]any{"type": "integer"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_market_breadth",
				Description: "Market center breadth for cn/hk/us equities",
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
	case "get_quote":
		return r.getQuote(args)
	case "get_snapshot_summary":
		return r.getSnapshotSummary(args)
	case "get_klines_summary":
		return r.getKlinesSummary(args)
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
	return marshal(map[string]any{
		"ok": true, "symbol": resp.Symbol, "interval": resp.Interval, "source": resp.Source,
		"candleCount": len(resp.Candles),
		"open": first.Open, "close": last.Close, "high": high, "low": low,
		"rangeChangePct": round2(chg),
		"firstOpenTime":  first.Time,
		"lastOpenTime":   last.Time,
	})
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
