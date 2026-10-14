package tools

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
)

type Outcome struct {
	OK       bool
	Status   evidence.Status
	Summary  string
	Evidence []evidence.Evidence
}

// ParseOutcome checks business status as well as transport/execution errors.
func ParseOutcome(raw string, execErr error) Outcome {
	out := Outcome{Status: evidence.Unavailable}
	if execErr != nil {
		out.Summary = execErr.Error()
		return out
	}
	var payload struct {
		OK       bool                `json:"ok"`
		Status   evidence.Status     `json:"status"`
		Error    string              `json:"error"`
		Evidence []evidence.Evidence `json:"evidence"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		out.Summary = "invalid tool JSON result"
		return out
	}
	out.Status = payload.Status
	if out.Status == "" {
		out.Status = evidence.Unavailable
		if payload.OK {
			out.Status = evidence.OK
		}
	}
	switch out.Status {
	case evidence.OK, evidence.Partial, evidence.Unavailable, evidence.Stale:
	default:
		out.Status = evidence.Unavailable
	}
	// A declared failure can never be promoted by status alone.
	if out.Status == evidence.OK && !payload.OK {
		out.Status = evidence.Unavailable
	}
	out.OK = payload.OK && out.Status == evidence.OK
	out.Evidence = payload.Evidence
	var node map[string]any
	_ = json.Unmarshal([]byte(raw), &node)
	summary, _ := json.Marshal(map[string]any{
		"status": out.Status, "error": payload.Error, "evidenceCount": len(out.Evidence),
		"issues": resultIssues(node, ""), "limitations": node["limitations"],
	})
	out.Summary = string(summary)
	return out
}

func resultIssues(node map[string]any, path string) []map[string]any {
	var issues []map[string]any
	if status := asString(node["status"]); status != "" && status != "ok" {
		issues = append(issues, map[string]any{"path": path, "status": status,
			"error": node["error"], "hint": node["hint"]})
	}
	for _, key := range []string{"quote", "kline1d", "news"} {
		if child, ok := node[key].(map[string]any); ok {
			issues = append(issues, resultIssues(child, path+"/"+key)...)
		}
	}
	for _, key := range []string{"items", "indices", "stocks"} {
		if rows, ok := node[key].([]any); ok {
			for i, value := range rows {
				if child, ok := value.(map[string]any); ok {
					issues = append(issues, resultIssues(child, fmt.Sprintf("%s/%s/%d", path, key, i))...)
				}
			}
		}
	}
	return issues
}

func decorateResult(kind, raw string, now time.Time) (string, error) {
	var node map[string]any
	if err := json.Unmarshal([]byte(raw), &node); err != nil || node == nil {
		return "", fmt.Errorf("invalid tool result JSON")
	}
	decorateNode(kind, node, now)
	return marshal(node)
}

func decorateNode(kind string, node map[string]any, now time.Time) []evidence.Evidence {
	var records []evidence.Evidence
	var children []evidence.Status
	for _, key := range []string{"quote", "kline1d", "news"} {
		if child, ok := node[key].(map[string]any); ok {
			records = append(records, decorateNode(key, child, now)...)
			children = append(children, evidence.Status(asString(child["status"])))
		}
	}
	for _, key := range []string{"items", "indices", "stocks"} {
		if rows, ok := node[key].([]any); ok {
			for _, row := range rows {
				if child, ok := row.(map[string]any); ok {
					if _, structured := child["ok"]; structured {
						records = append(records, decorateNode("item", child, now)...)
						children = append(children, evidence.Status(asString(child["status"])))
					}
				}
			}
		}
	}
	status := evidence.Status(asString(node["status"]))
	if status == "" {
		status = evidence.OK
		if node["ok"] != true {
			status = evidence.Unavailable
		}
	}
	if len(children) > 0 && status == evidence.OK {
		status = combineStatuses(children)
	}
	if len(children) == 0 && status != evidence.Unavailable {
		if kind == "get_express_news" || kind == "news" {
			if rows, ok := node["items"].([]any); !ok || len(rows) == 0 {
				status = evidence.Partial
			}
		}
		if _, quote := node["price"]; quote || node["priceUsdt"] != nil {
			kind = "quote"
		}
		if node["candleCount"] != nil {
			kind = "klines"
		}
		if node["eventStatus"] != nil {
			kind = "market_event"
		}
		if kind == "get_express_news" || kind == "news" {
			if rows, ok := node["items"].([]any); ok {
				for _, row := range rows {
					if item, ok := row.(map[string]any); ok {
						copy := cloneMap(item)
						for _, field := range []string{"source", "fetchedAt", "assetClass", "instrumentId", "symbol"} {
							if field != "source" || copy[field] == nil {
								copy[field] = node[field]
							}
						}
						copy["updatedAt"] = item["publishTime"]
						e := buildEvidence("news", copy, status, now)
						records = append(records, e)
						if e.Status != evidence.OK && status == evidence.OK {
							status = evidence.Partial
						}
					}
				}
			}
		} else {
			e := buildEvidence(kind, node, status, now)
			records = append(records, e)
			status = e.Status
		}
	}
	if flag, ok := node["partial"].(bool); ok && flag && status == evidence.OK {
		status = evidence.Partial
	}
	node["status"], node["ok"] = string(status), status == evidence.OK
	if len(records) > 0 {
		node["evidence"] = records
	}
	return records
}

func combineStatuses(children []evidence.Status) evidence.Status {
	allOK, allUnavailable, allStale := true, true, true
	for _, status := range children {
		allOK = allOK && status == evidence.OK
		allUnavailable = allUnavailable && status == evidence.Unavailable
		allStale = allStale && status == evidence.Stale
	}
	switch {
	case allOK:
		return evidence.OK
	case allUnavailable:
		return evidence.Unavailable
	case allStale:
		return evidence.Stale
	default:
		return evidence.Partial
	}
}

func buildEvidence(kind string, node map[string]any, status evidence.Status, now time.Time) evidence.Evidence {
	e := evidence.Evidence{
		Kind: kind, InstrumentID: asString(node["instrumentId"]), Symbol: asString(node["symbol"]),
		AssetClass: asString(node["assetClass"]), InstrumentType: asString(node["instrumentType"]),
		Source: asString(node["source"]), Currency: asString(node["currency"]),
		ChangeBasis: asString(node["changeBasis"]), Status: status,
		URL: asString(node["url"]), Facts: map[string]any{},
		Limitations: asStringSlice(node["limitations"]),
	}
	e.AsOf = parseTime(node["updatedAt"])
	e.AsOfBasis = "data_time"
	if kind == "news" {
		e.AsOfBasis = "publication_time"
	}
	if e.AsOf.IsZero() {
		e.AsOf = parseTime(node["lastOpenTime"])
		if !e.AsOf.IsZero() {
			e.AsOfBasis = "candle_open"
		}
	}
	if e.Source == "" {
		if e.Status == evidence.OK {
			e.Status = evidence.Partial
		}
		e.Limitations = append(e.Limitations, "来源未知，不可视为已验证数据")
	}
	if e.AsOf.IsZero() {
		e.AsOf = parseTime(node["ts"])
		if !e.AsOf.IsZero() {
			e.AsOfBasis = "collection_time"
		}
	}
	e.FetchedAt = parseTime(node["fetchedAt"])
	if e.AsOf.IsZero() && node["breadth"] != nil && !e.FetchedAt.IsZero() {
		e.AsOf, e.AsOfBasis = e.FetchedAt, "collection_time"
	}
	if e.FetchedAt.IsZero() {
		e.FetchedAt = e.AsOf
	}
	e.ExpiresAt = parseTime(node["expiresAt"])
	if e.ExpiresAt.IsZero() && !e.FetchedAt.IsZero() {
		e.ExpiresAt = e.FetchedAt.Add(5 * time.Minute)
	}
	if e.AsOf.IsZero() {
		e.AsOfBasis = "unknown"
		if e.Status == evidence.OK {
			e.Status = evidence.Partial
		}
		e.Limitations = append(e.Limitations, "数据时点未知，不可作为最新数据")
	}
	if e.AsOfBasis == "collection_time" {
		e.Limitations = append(e.Limitations, "采集时间不代表每项指标的数据时间；独立 updatedAt 未知的子项不可宣称最新")
	}
	if e.AsOf.After(now.Add(time.Minute)) || !e.FetchedAt.IsZero() && (e.FetchedAt.After(now.Add(time.Minute)) || !now.Before(e.ExpiresAt)) {
		e.Status = evidence.Stale
		e.Limitations = append(e.Limitations, "证据已过期或时间异常，仅供历史参考")
	}
	for _, key := range []string{
		"price", "priceUsdt", "changePct", "change24hPct", "volume", "markPrice", "indexPrice", "fundingRate",
		"interval", "pair", "candleCount", "firstOpenTime", "lastOpenTime", "open", "close", "high", "low",
		"rangeChangePct", "lastBarChangePct", "sma7", "sma30", "closeVsSma7", "closeVsSma30",
		"volumeRatio", "volumeNote", "nearRangeHigh", "nearRangeLow", "lastCandleClosed",
		"title", "body", "publishTime", "matchedBy", "entities", "sessionNote",
		"market", "marketActive", "breadth", "hotSectors", "heatmapTop", "fundflow",
		"macro", "rates", "topGainers", "topLosers", "quoteCount", "indexCount",
		"overview", "holdings", "holdingCount", "omittedHoldings", "id", "severity", "eventStatus", "symbols", "startTime", "signals", "hours",
	} {
		if value, ok := node[key]; ok {
			e.Facts[key] = value
		}
	}
	e.Seal()
	return e
}

func parseTime(value any) time.Time {
	switch v := value.(type) {
	case string:
		t, _ := time.Parse(time.RFC3339Nano, v)
		return t
	case float64:
		if v <= 0 {
			return time.Time{}
		}
		if v > 1e12 {
			return time.UnixMilli(int64(v)).UTC()
		}
		return time.Unix(int64(v), 0).UTC()
	}
	return time.Time{}
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
