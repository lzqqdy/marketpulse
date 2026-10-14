package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/event"
	"github.com/lzqqdy/marketpulse/internal/portfolio"
)

type PortfolioReader interface {
	Enabled() bool
	Overview(context.Context, int64) (portfolio.Overview, error)
	GetHoldings(context.Context, int64) (portfolio.HoldingsResult, error)
}

type EventReader interface {
	Enabled() bool
	List(context.Context, event.ListFilter) (event.ListResult, error)
}

// WithReaders and ForUser return request-local copies; never mutate shared identity.
func (r *Registry) WithReaders(p PortfolioReader, e EventReader) *Registry {
	copy := *r
	copy.portfolio = p
	copy.events = e
	return &copy
}

func (r *Registry) ForUser(id int64) *Registry {
	copy := *r
	copy.userID = id
	return &copy
}

func readOnlyDefinitions() []Definition {
	return []Definition{
		{Type: "function", Function: FunctionSchema{Name: "get_portfolio_overview", Description: "只读当前登录用户的资产概览、收益口径及最多20条持仓。不能读取他人数据或修改资产。", Parameters: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}},
		{Type: "function", Function: FunctionSchema{Name: "get_market_events", Description: "只读市场事件检测结果（当前仅加密），默认最近24小时；检测不是新闻或价格变化因果。", Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"symbol": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 10}, "hours": map[string]any{"type": "integer", "minimum": 1, "maximum": 168},
		}, "additionalProperties": false}}},
	}
}

func (r *Registry) getPortfolioOverview(ctx context.Context, args map[string]any) (string, error) {
	if len(args) != 0 {
		return "", fmt.Errorf("get_portfolio_overview accepts no identity or other arguments")
	}
	if r.userID <= 0 || r.portfolio == nil || !r.portfolio.Enabled() {
		return marshal(map[string]any{"ok": false, "error": "本人资产读取不可用（未登录或资产模块未开启）"})
	}
	overview, err := r.portfolio.Overview(ctx, r.userID)
	if err != nil {
		return "", err
	}
	holdings, err := r.portfolio.GetHoldings(ctx, r.userID)
	if err != nil {
		return "", err
	}
	count := len(holdings.Holdings)
	rows := holdings.Holdings
	if len(rows) > 20 {
		rows = rows[:20]
	}
	partial := len(overview.StaleSymbols) > 0 || len(overview.MissingSymbols) > 0 || overview.RateFallback || holdings.RateFallback || len(holdings.StaleSymbols) > 0 || len(holdings.MissingSymbols) > 0
	for _, row := range rows {
		partial = partial || row.Missing || row.Stale || row.PriceUpdatedAt.IsZero()
	}
	return marshal(map[string]any{
		"ok": true, "partial": partial, "source": "portfolio", "ts": r.now(), "fetchedAt": r.now(), "currency": "CNY", "changeBasis": "portfolio_valuation",
		"overview": overview, "holdings": rows, "holdingCount": count, "omittedHoldings": count - len(rows),
		"limitations": []string{"资产服务计算时点不等于每条行情的数据时间；逐项检查 priceUpdatedAt/missing/stale；汇率数据时间未单独提供", "收益为估值与本金/历史快照之差，不是出入金与交易流水调整后的投资回报率", "alpha 类型持仓为合约/代币化参考，不是美股现货；不得据此下单或改仓"},
	})
}

func (r *Registry) getMarketEvents(ctx context.Context, args map[string]any) (string, error) {
	for key := range args {
		if key != "symbol" && key != "limit" && key != "hours" {
			return "", fmt.Errorf("unsupported event filter: %s", key)
		}
	}
	if r.events == nil || !r.events.Enabled() {
		return marshal(map[string]any{"ok": false, "error": "市场事件模块未开启"})
	}
	limit, hours := asInt(args["limit"], 5), asInt(args["hours"], 24)
	if limit < 1 || limit > 10 || hours < 1 || hours > 168 {
		return "", fmt.Errorf("event limit must be 1..10 and hours 1..168")
	}
	symbol := normalizeCryptoSymbol(asString(args["symbol"]))
	if len(symbol) > 30 || strings.ContainsAny(symbol, " /\\") {
		return "", fmt.Errorf("invalid event symbol")
	}
	now := r.now()
	start := now.Add(-time.Duration(hours) * time.Hour)
	result, err := r.events.List(ctx, event.ListFilter{Symbol: symbol, Start: &start, End: &now, Limit: limit})
	if err != nil {
		return "", err
	}
	rows := make([]map[string]any, 0, limit)
	for _, item := range result.Items {
		if len(rows) >= limit {
			break
		}
		signals := item.Signals
		if len(signals) > 5 {
			signals = signals[:5]
		}
		facts := make([]map[string]any, 0, len(signals))
		for _, signal := range signals {
			facts = append(facts, map[string]any{"signalType": signal.SignalType, "symbol": signal.Symbol, "value": signal.Value, "baseline": signal.Baseline, "ratio": signal.Ratio, "changePct": signal.ChangePct, "window": signal.Window, "timestamp": signal.Timestamp})
		}
		rows = append(rows, map[string]any{"ok": true, "source": "event_engine", "id": item.ID, "title": item.Title, "severity": item.Severity, "eventStatus": item.Status, "symbols": item.Symbols, "startTime": item.StartTime, "updatedAt": item.UpdatedAt, "fetchedAt": now, "signals": facts, "limitations": []string{"规则检测事件，不是新闻或涨跌因果；信号时间以各项 timestamp 为准"}})
	}
	return marshal(map[string]any{"ok": true, "source": "event_engine", "ts": now, "fetchedAt": now, "symbol": symbol, "hours": hours, "items": rows, "limitations": []string{"事件是系统规则检测结果，不是新闻或涨跌原因；事件数据时间按 updatedAt 查看", "空列表仅表示查询窗口内未检出匹配记录，不代表市场没有风险"}})
}
