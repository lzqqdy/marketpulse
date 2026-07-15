package alerts

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// triggerMeta carries live quote context for notification copy.
type triggerMeta struct {
	Price       float64
	ChangePct   float64 // day change %
	DisplayName string
	WindowAmp   float64 // type5 amplitude %, optional
}

// Known Chinese display names for indices / commodities (never show raw ids like ks11).
var indexDisplayNames = map[string]string{
	"sh000001": "上证",
	"sz399001": "深证",
	"sz399006": "创业板",
	"sh000300": "沪深300",
	"sh000688": "科创50",
	"hsi":      "恒生",
	"dji":      "道琼斯",
	"ixic":     "纳斯达克",
	"gspc":     "标普500",
	"n225":     "日经225",
	"ks11":     "韩国综指",
	"gold":     "国际黄金",
	"silver":   "国际白银",
	"crude":    "WTI原油",
}

// Spot tickers stay as BTC / ETH (matches user title example 【BTC 上涨】).
var spotDisplayNames = map[string]string{
	"BTC": "BTC",
	"ETH": "ETH",
	"BNB": "BNB",
	"LTC": "LTC",
	"FIL": "FIL",
}

func displayName(assetType, symbol, liveName string) string {
	live := strings.TrimSpace(liveName)
	if assetType == AssetIndex {
		id := strings.ToLower(strings.TrimSpace(symbol))
		if cn, ok := indexDisplayNames[id]; ok {
			return cn
		}
		if live != "" && !looksLikeRawID(live) {
			return live
		}
		if id != "" {
			return id
		}
		return live
	}
	base := normalizeSpotBase(symbol)
	if n, ok := spotDisplayNames[base]; ok {
		return n
	}
	if live != "" && !looksLikeRawID(live) {
		return live
	}
	if base != "" {
		return base
	}
	return strings.TrimSpace(symbol)
}

func looksLikeRawID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	lower := strings.ToLower(s)
	if _, ok := indexDisplayNames[lower]; ok {
		return true
	}
	if strings.EqualFold(s, "KOSPI") {
		return true
	}
	if strings.HasPrefix(lower, "sh") || strings.HasPrefix(lower, "sz") {
		return true
	}
	return false
}

func actionVerb(ruleType int, price float64, params RuleParams) string {
	switch ruleType {
	case 1:
		return "上涨"
	case 2:
		return "下跌"
	case 3, 4:
		if params.Upper != nil && price >= *params.Upper {
			return "冲高"
		}
		if params.Lower != nil && price <= *params.Lower {
			return "回落"
		}
		return "破位"
	case 5:
		return "剧震"
	default:
		return "告警"
	}
}

func formatPct(pct float64) string {
	sign := ""
	if pct > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.2f%%", sign, pct)
}

func formatPrice(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "-"
	}
	abs := math.Abs(v)
	switch {
	case abs >= 1000:
		return strconv.FormatFloat(v, 'f', 2, 64)
	case abs >= 1:
		return strconv.FormatFloat(v, 'f', 4, 64)
	default:
		return strconv.FormatFloat(v, 'f', 6, 64)
	}
}

func parseSetPrice(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func formatAlertMessage(rule Rule, triggerValue float64, meta triggerMeta) (title, body string) {
	name := displayName(rule.AssetType, rule.Symbol, meta.DisplayName)
	price := meta.Price
	if price <= 0 && rule.RuleType != 5 {
		price = triggerValue
	}
	if price <= 0 {
		price = triggerValue
	}

	verb := actionVerb(rule.RuleType, price, rule.Params)
	changePct := meta.ChangePct
	if rule.RuleType == 5 && meta.WindowAmp > 0 {
		// Title trailing % uses 5m amplitude for 剧震.
		title = fmt.Sprintf("【%s %s】%s 振幅%s", name, verb, formatPrice(price), formatPct(meta.WindowAmp))
	} else {
		title = fmt.Sprintf("【%s %s】%s %s", name, verb, formatPrice(price), formatPct(changePct))
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("现价 %s（日涨跌 %s）", formatPrice(price), formatPct(changePct)))

	switch rule.RuleType {
	case 1:
		if rule.Params.Target != nil {
			lines = append(lines, fmt.Sprintf("已涨破目标 %s", formatPrice(*rule.Params.Target)))
		}
	case 2:
		if rule.Params.Target != nil {
			lines = append(lines, fmt.Sprintf("已跌破目标 %s", formatPrice(*rule.Params.Target)))
		}
	case 3, 4:
		if rule.Params.Upper != nil && price >= *rule.Params.Upper {
			lines = append(lines, fmt.Sprintf("已触及上沿 %s", formatPrice(*rule.Params.Upper)))
		} else if rule.Params.Lower != nil && price <= *rule.Params.Lower {
			lines = append(lines, fmt.Sprintf("已触及下沿 %s", formatPrice(*rule.Params.Lower)))
		} else {
			lines = append(lines, "已触及区间边界")
		}
		if rule.Params.Upper != nil && rule.Params.Lower != nil {
			lines = append(lines, fmt.Sprintf("区间 %s ~ %s", formatPrice(*rule.Params.Lower), formatPrice(*rule.Params.Upper)))
		}
	case 5:
		amp := triggerValue
		if meta.WindowAmp > 0 {
			amp = meta.WindowAmp
		}
		lines = append(lines, fmt.Sprintf("近5分钟振幅 %s", formatPct(amp)))
		if rule.Params.RapidChg != nil {
			lines = append(lines, fmt.Sprintf("阈值 ≥ %s", formatPct(*rule.Params.RapidChg)))
		}
	}

	if set, ok := parseSetPrice(rule.SetPrice); ok && set > 0 && price > 0 {
		vsSet := (price - set) / set * 100
		lines = append(lines, fmt.Sprintf("相对设定价 %s：%s", formatPrice(set), formatPct(vsSet)))
	}

	switch rule.Frequency {
	case FrequencyOnce:
		lines = append(lines, "频率：仅一次")
	case FrequencyDaily:
		lines = append(lines, "频率：每日一次")
	case FrequencyLoop:
		mins := rule.IntervalMinutes
		if mins <= 0 {
			mins = 10
		}
		lines = append(lines, fmt.Sprintf("频率：每 %d 分钟可再推", mins))
	}

	body = strings.Join(lines, "\n")
	return title, body
}
