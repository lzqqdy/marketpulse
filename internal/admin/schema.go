package admin

import (
	"regexp"
	"strings"
)

// FieldSchema describes how the frontend should render a config path.
type FieldSchema struct {
	Path        string   `json:"path"`
	Widget      string   `json:"widget"` // switch|select|text|password|number|duration|string_list|object_list|textarea
	Options     []string `json:"options,omitempty"`
	AllowCustom bool     `json:"allowCustom,omitempty"`
	Label       string   `json:"label,omitempty"`
	// ItemFields drives object_list form rows (e.g. id/name/symbol).
	ItemFields []ItemField `json:"itemFields,omitempty"`
}

// ItemField is one column in an object_list row editor.
type ItemField struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
}

// SectionMeta groups top-level keys in the form UI.
type SectionMeta struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Keys  []string `json:"keys"`
}

// DefaultSections returns form tab grouping.
func DefaultSections() []SectionMeta {
	return []SectionMeta{
		{ID: "basic", Label: "基础", Keys: []string{"app", "cors", "symbols"}},
		{ID: "platform", Label: "平台", Keys: []string{"mysql", "redis", "upload"}},
		{ID: "business", Label: "业务", Keys: []string{"users", "alerts", "portfolio", "ai", "event", "smtp"}},
		{ID: "ingest", Label: "采集", Keys: []string{"ingest", "alpha"}},
	}
}

var knownSchemas = []FieldSchema{
	{Path: "app.mode", Widget: "select", Options: []string{"debug", "release"}, Label: "运行模式"},
	{Path: "app.addr", Widget: "text", Label: "监听地址"},
	{Path: "app.log_dir", Widget: "text", Label: "日志目录"},
	{Path: "app.static_dir", Widget: "text", Label: "静态目录"},

	{Path: "cors.allowed_origins", Widget: "string_list", Label: "CORS 来源"},

	{Path: "mysql.enabled", Widget: "switch"},
	{Path: "mysql.host", Widget: "text"},
	{Path: "mysql.port", Widget: "number"},
	{Path: "mysql.user", Widget: "text"},
	{Path: "mysql.password", Widget: "password"},
	{Path: "mysql.database", Widget: "text"},
	{Path: "mysql.params", Widget: "text"},
	{Path: "mysql.max_open_conns", Widget: "number"},
	{Path: "mysql.max_idle_conns", Widget: "number"},
	{Path: "mysql.conn_max_lifetime", Widget: "duration"},
	{Path: "mysql.conn_max_idle_time", Widget: "duration"},

	{Path: "redis.enabled", Widget: "switch"},
	{Path: "redis.addr", Widget: "text"},
	{Path: "redis.password", Widget: "password"},
	{Path: "redis.db", Widget: "number"},
	{Path: "redis.pool_size", Widget: "number"},
	{Path: "redis.min_idle_conns", Widget: "number"},
	{Path: "redis.dial_timeout", Widget: "duration"},
	{Path: "redis.read_timeout", Widget: "duration"},
	{Path: "redis.write_timeout", Widget: "duration"},

	{Path: "users.enabled", Widget: "switch"},
	{Path: "users.auto_migrate", Widget: "switch"},
	{Path: "users.session_ttl", Widget: "duration"},
	{Path: "users.seed.phone", Widget: "text"},
	{Path: "users.seed.password", Widget: "password"},
	{Path: "users.seed.display_name", Widget: "text"},

	{Path: "alerts.enabled", Widget: "switch"},
	{Path: "alerts.auto_migrate", Widget: "switch"},
	{Path: "alerts.daily_timezone", Widget: "text"},
	{Path: "alerts.loop_interval_min", Widget: "number"},
	{Path: "alerts.loop_interval_max", Widget: "number"},
	{Path: "alerts.inbox_max_len", Widget: "number"},

	{Path: "portfolio.enabled", Widget: "switch"},
	{Path: "portfolio.auto_migrate", Widget: "switch"},
	{Path: "portfolio.daily_timezone", Widget: "text"},
	{Path: "portfolio.default_usdt_cny", Widget: "number"},

	{Path: "ai.enabled", Widget: "switch"},
	{Path: "ai.auto_migrate", Widget: "switch"},
	{Path: "ai.provider", Widget: "select", Options: []string{"deepseek"}, AllowCustom: true},
	{Path: "ai.base_url", Widget: "text"},
	{Path: "ai.api_key", Widget: "password"},
	{Path: "ai.model", Widget: "select", Options: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, AllowCustom: true},
	{Path: "ai.timeout", Widget: "duration"},
	{Path: "ai.max_tool_rounds", Widget: "number"},
	{Path: "ai.max_history_messages", Widget: "number"},
	{Path: "ai.daily_quota_per_user", Widget: "number"},
	{Path: "ai.system_prompt", Widget: "textarea"},

	{Path: "event.enabled", Widget: "switch", Label: "市场异动开关"},
	{Path: "event.auto_migrate", Widget: "switch", Label: "自动建表"},
	{Path: "event.evaluate_interval", Widget: "duration", Label: "检测间隔"},
	{Path: "event.aggregate_window", Widget: "duration", Label: "聚合时间窗"},
	{Path: "event.kline_timeout", Widget: "duration", Label: "K线超时"},
	{Path: "event.symbols", Widget: "string_list", Label: "监控标的"},
	{Path: "event.price.5m.threshold_pct", Widget: "number", Label: "价格阈值 5m(%)"},
	{Path: "event.price.15m.threshold_pct", Widget: "number", Label: "价格阈值 15m(%)"},
	{Path: "event.price.1h.threshold_pct", Widget: "number", Label: "价格阈值 1h(%)"},
	{Path: "event.volume.lookback_bars", Widget: "number", Label: "成交量回看根数"},
	{Path: "event.volume.ratio_medium", Widget: "number", Label: "放量倍数(触发)"},
	{Path: "event.volume.ratio_high", Widget: "number", Label: "放量倍数(高)"},
	{Path: "event.volume.ratio_extreme", Widget: "number", Label: "放量倍数(极高)"},
	{Path: "event.volatility.lookback_bars", Widget: "number", Label: "波动回看根数"},
	{Path: "event.volatility.ratio", Widget: "number", Label: "波动倍数阈值"},
	{Path: "event.liquidation.sample_interval", Widget: "duration", Label: "爆仓采样间隔"},
	{Path: "event.liquidation.baseline_samples", Widget: "number", Label: "爆仓基线样本数"},
	{Path: "event.liquidation.ratio", Widget: "number", Label: "爆仓倍数阈值"},

	{Path: "smtp.host", Widget: "text"},
	{Path: "smtp.port", Widget: "number"},
	{Path: "smtp.username", Widget: "text"},
	{Path: "smtp.password", Widget: "password"},
	{Path: "smtp.from", Widget: "text"},

	{Path: "upload.dir", Widget: "text"},
	{Path: "upload.public_path", Widget: "text"},
	{Path: "upload.max_avatar_bytes", Widget: "number"},

	{Path: "symbols", Widget: "string_list"},

	{Path: "alpha.enabled", Widget: "switch"},
	{Path: "alpha.provider", Widget: "select", Options: []string{"bitget", "binance"}},
	{Path: "alpha.product_type", Widget: "text"},
	{Path: "alpha.quote_asset", Widget: "text"},
	{Path: "alpha.poll_interval", Widget: "duration"},
	{Path: "alpha.resolve_interval", Widget: "duration"},
	{Path: "alpha.indices", Widget: "object_list", Label: "指数标的", ItemFields: []ItemField{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "名称"},
		{Key: "symbol", Label: "交易对"},
	}},
	{Path: "alpha.stocks", Widget: "object_list", Label: "股票标的", ItemFields: []ItemField{
		{Key: "id", Label: "ID"},
		{Key: "name", Label: "名称"},
		{Key: "symbol", Label: "交易对"},
	}},

	{Path: "ingest.binance.ws_base", Widget: "text"},
	{Path: "ingest.baidu.enabled", Widget: "switch"},
	{Path: "ingest.baidu.base_url", Widget: "text"},
	{Path: "ingest.baidu.ws_url", Widget: "text"},
	{Path: "ingest.baidu.ws_enabled", Widget: "switch"},
	{Path: "ingest.baidu.ws_reconnect_max", Widget: "number"},
	{Path: "ingest.baidu.ws_reconnect_delay", Widget: "duration"},
	{Path: "ingest.baidu.ws_patch_interval", Widget: "duration"},
	{Path: "ingest.otc.usdt_cny_interval", Widget: "duration"},
	{Path: "ingest.forex.usd_cny_interval", Widget: "duration"},
	{Path: "ingest.equity.interval", Widget: "duration"},
	{Path: "ingest.equity.providers", Widget: "string_list", Options: []string{"baidu", "tencent", "eastmoney"}},
	{Path: "ingest.equity.index_ids", Widget: "string_list"},
	{Path: "ingest.macro.interval", Widget: "duration"},
}

var commentEnumRe = regexp.MustCompile(`#\s*([A-Za-z0-9_.\-]+(?:\s*\|\s*[A-Za-z0-9_.\-]+)+)\s*$`)

// BuildSchema merges known schemas with type inference and comment enums from example YAML text.
func BuildSchema(exampleTree map[string]any, exampleYAML string) []FieldSchema {
	byPath := map[string]FieldSchema{}
	for _, s := range knownSchemas {
		byPath[s.Path] = s
	}

	// Parse line-end enums from example comments: key: "value" # a | b
	for _, line := range strings.Split(exampleYAML, "\n") {
		m := commentEnumRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := extractYAMLKey(line)
		if key == "" {
			continue
		}
		opts := splitEnumOptions(m[1])
		if len(opts) == 0 {
			continue
		}
		// Only attach to known leaf paths ending with this key when unique enough;
		// also update matching known path suffixes.
		for path, sch := range byPath {
			if strings.HasSuffix(path, "."+key) || path == key {
				if len(sch.Options) == 0 {
					sch.Options = opts
					sch.Widget = "select"
					byPath[path] = sch
				}
			}
		}
		if key == "mode" {
			sch := byPath["app.mode"]
			sch.Options = opts
			sch.Widget = "select"
			byPath["app.mode"] = sch
		}
	}

	inferSchema("", exampleTree, byPath)

	out := make([]FieldSchema, 0, len(byPath))
	for _, s := range byPath {
		out = append(out, s)
	}
	return out
}

func inferSchema(prefix string, tree map[string]any, byPath map[string]FieldSchema) {
	for key, val := range tree {
		path := joinPath(prefix, key)
		if _, exists := byPath[path]; exists {
			if m, ok := val.(map[string]any); ok {
				inferSchema(path, m, byPath)
			}
			continue
		}
		switch t := val.(type) {
		case map[string]any:
			inferSchema(path, t, byPath)
		case []any:
			widget := "string_list"
			var itemFields []ItemField
			if len(t) > 0 {
				if m, ok := t[0].(map[string]any); ok {
					widget = "object_list"
					itemFields = itemFieldsFromMap(m)
				}
			}
			byPath[path] = FieldSchema{Path: path, Widget: widget, ItemFields: itemFields}
		case bool:
			byPath[path] = FieldSchema{Path: path, Widget: "switch"}
		case int, int64, float64, float32:
			byPath[path] = FieldSchema{Path: path, Widget: "number"}
		case string:
			widget := "text"
			lk := strings.ToLower(key)
			if strings.Contains(lk, "password") || strings.Contains(lk, "api_key") || lk == "api_key" {
				widget = "password"
			} else if strings.HasSuffix(lk, "interval") || strings.HasSuffix(lk, "timeout") || strings.HasSuffix(lk, "ttl") || strings.HasSuffix(lk, "lifetime") {
				widget = "duration"
			}
			byPath[path] = FieldSchema{Path: path, Widget: widget}
		default:
			byPath[path] = FieldSchema{Path: path, Widget: "text"}
		}
	}
}

func extractYAMLKey(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return ""
	}
	return strings.TrimSpace(line[:i])
}

func splitEnumOptions(raw string) []string {
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func itemFieldsFromMap(m map[string]any) []ItemField {
	prefer := []string{"id", "name", "symbol"}
	seen := map[string]bool{}
	out := make([]ItemField, 0, len(m))
	for _, k := range prefer {
		if _, ok := m[k]; ok {
			out = append(out, ItemField{Key: k, Label: k})
			seen[k] = true
		}
	}
	for k, v := range m {
		if seen[k] {
			continue
		}
		switch v.(type) {
		case map[string]any, []any:
			continue
		default:
			out = append(out, ItemField{Key: k, Label: k})
		}
	}
	return out
}
