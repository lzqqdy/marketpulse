package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/lzqqdy/marketpulse/internal/marketdata/expressnews"
)

func (r *Registry) getExpressNews(ctx context.Context, args map[string]any) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	limit := asInt(args["limit"], 8)
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	tag, symbol := asString(args["tag"]), asString(args["symbol"])
	rn := limit
	var inst instrument
	if symbol != "" {
		var err error
		inst, err = r.resolveInstrument(symbol, asString(args["assetClass"]))
		if err != nil {
			return marshal(map[string]any{"ok": false, "error": err.Error()})
		}
		tag, rn = inst.NewsTag, 20
	}
	resp, err := r.md.ExpressNews(tag, 1, rn, 0)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error(), "tag": tag})
	}
	items := make([]map[string]any, 0, limit)
	seen := make(map[string]bool)
	for _, it := range resp.Items {
		matchedBy := "market"
		if symbol != "" {
			matchedBy = matchNews(it, inst)
			if matchedBy == "" {
				continue
			}
		}
		key := it.ID
		if key == "" {
			key = fmt.Sprintf("%s|%s|%d", it.Title, it.Body, it.PublishTime)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		body := []rune(it.Body)
		if len(body) > 120 {
			body = append(body[:120], []rune("…")...)
		}
		item := map[string]any{
			"id": it.ID, "title": it.Title, "body": string(body), "publishTime": it.PublishTime,
			"tag": it.Tag, "important": it.Important, "matchedBy": matchedBy,
		}
		if it.Provider != "" {
			item["source"] = it.Provider
		}
		if u, parseErr := url.Parse(it.ThirdURL); parseErr == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") {
			item["url"] = u.String()
		}
		if len(it.Entities) > 0 {
			item["entities"] = it.Entities
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	out := map[string]any{
		"ok": true, "tag": tag, "source": resp.Source, "fetchedAt": resp.FetchedAt,
		"items": items, "searchedCount": len(resp.Items),
		"limitations": []string{"新闻匹配仅表示相关，不证明该新闻导致价格变化"},
	}
	if symbol != "" {
		out["symbol"], out["assetClass"], out["instrumentId"] = inst.Symbol, inst.Asset, inst.ID
	}
	if len(items) == 0 {
		out["partial"] = true
		out["hint"] = "当前获取的最近新闻中没有匹配内容；未用不相关新闻补齐，也不代表历史上没有相关新闻"
	}
	return marshal(out)
}

func matchNews(item expressnews.NewsItem, inst instrument) string {
	for _, entity := range item.Entities {
		if (inst.Asset == "us_equity" || inst.Asset == "us_contract") && entity.Market != "" && entity.Market != "us" && entity.Market != "美股" {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(entity.Code))
		code = strings.TrimPrefix(code, "S_US")
		code = strings.TrimPrefix(code, "US-")
		if i := strings.LastIndex(code, ":"); i >= 0 {
			code = code[i+1:]
		}
		if dot := strings.Index(code, "."); dot >= 0 {
			if strings.HasPrefix(code, "US.") {
				code = code[3:]
			} else {
				code = code[:dot]
			}
		}
		if code == inst.Symbol || strings.EqualFold(entity.Code, inst.ID) {
			return "entity"
		}
		for _, alias := range inst.Aliases {
			if alias != "" && strings.EqualFold(entity.Name, alias) {
				return "entity"
			}
		}
	}
	text := strings.ToLower(item.Title + "\n" + item.Body)
	for _, alias := range inst.Aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		if containsTerm(text, alias) {
			return "text"
		}
	}
	return ""
}

// Latin tickers must be complete tokens: META must not match "metaverse".
func containsTerm(text, term string) bool {
	for start := 0; start < len(text); {
		pos := strings.Index(text[start:], term)
		if pos < 0 {
			return false
		}
		pos += start
		end := pos + len(term)
		leftOK := !asciiWord(term[0]) || pos == 0 || !asciiWord(text[pos-1])
		rightOK := !asciiWord(term[len(term)-1]) || end == len(text) || !asciiWord(text[end])
		if leftOK && rightOK {
			return true
		}
		start = end
	}
	return false
}

func asciiWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}
