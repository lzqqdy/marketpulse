package alerts

import (
	"strings"
	"testing"
)

func TestDisplayName_indexChinese(t *testing.T) {
	if got := displayName(AssetIndex, "ks11", "KOSPI"); got != "韩国综指" {
		t.Fatalf("got %q", got)
	}
	if got := displayName(AssetIndex, "sh000001", "上证"); got != "上证" {
		t.Fatalf("got %q", got)
	}
	if got := displayName(AssetIndex, "hsi", ""); got != "恒生" {
		t.Fatalf("got %q", got)
	}
}

func TestDisplayName_alphaPrefersChineseName(t *testing.T) {
	if got := displayName(AssetAlpha, "qqq", "纳指ETF"); got != "纳指ETF" {
		t.Fatalf("got %q", got)
	}
	if got := displayName(AssetAlpha, "aapl", ""); got != "AAPL" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatAlertMessage_titleShape(t *testing.T) {
	target := 100000.0
	rule := Rule{
		AssetType: AssetSpot,
		Symbol:    "BTCUSDT",
		RuleType:  1,
		Params:    RuleParams{Target: &target},
		SetPrice:  "98000",
		Frequency: FrequencyOnce,
	}
	title, body := formatAlertMessage(rule, 100123.4, triggerMeta{
		Price: 100123.4, ChangePct: 1.25, DisplayName: "BTC",
	})
	if !strings.HasPrefix(title, "【BTC 上涨】") {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(title, "+1.25%") {
		t.Fatalf("title missing pct: %q", title)
	}
	if !strings.Contains(body, "现价") || !strings.Contains(body, "已涨破目标") {
		t.Fatalf("body=%q", body)
	}
	if !strings.Contains(body, "相对设定价") {
		t.Fatalf("body missing set price: %q", body)
	}
}

func TestFormatAlertMessage_type5(t *testing.T) {
	th := 3.0
	rule := Rule{
		AssetType: AssetIndex,
		Symbol:    "ks11",
		RuleType:  5,
		Params:    RuleParams{RapidChg: &th},
		Frequency: FrequencyLoop,
		IntervalMinutes: 10,
	}
	title, body := formatAlertMessage(rule, 4.5, triggerMeta{
		Price: 2500, ChangePct: -0.3, DisplayName: "韩国综指", WindowAmp: 4.5,
	})
	if !strings.Contains(title, "【韩国综指 剧震】") {
		t.Fatalf("title=%q", title)
	}
	if strings.Contains(title, "ks11") {
		t.Fatalf("raw id leaked: %q", title)
	}
	if !strings.Contains(body, "近5分钟振幅") {
		t.Fatalf("body=%q", body)
	}
}
