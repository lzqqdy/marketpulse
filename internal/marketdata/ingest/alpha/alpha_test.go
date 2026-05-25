package alpha

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
)

func TestResolveAlphaSymbolFromAlphaID(t *testing.T) {
	item := config.AlphaItem{ID: "aaplon", Name: "AAPL", Symbol: "AAPLONUSDT"}
	docs := []any{
		[]any{
			map[string]any{
				"symbol":  "AAPLON",
				"name":    "Apple tokenized stock",
				"alphaId": "ALPHA_175",
			},
		},
	}
	got := resolveAlphaSymbol(item, "AAPLON", "USDT", docs)
	if got != "ALPHA_175USDT" {
		t.Fatalf("alpha symbol = %s", got)
	}
}

func TestResolveAlphaSymbolPrefersExactSymbolOverSubstring(t *testing.T) {
	item := config.AlphaItem{ID: "qqqon", Name: "QQQ", Symbol: "QQQONUSDT"}
	docs := []any{
		[]any{
			map[string]any{
				"symbol":  "PSQon",
				"name":    "ProShares Short QQQ (Ondo)",
				"alphaId": "ALPHA_908",
			},
			map[string]any{
				"symbol":  "TQQQon",
				"name":    "ProShares UltraPro QQQ (Ondo)",
				"alphaId": "ALPHA_852",
			},
			map[string]any{
				"symbol":  "QQQon",
				"name":    "Invesco QQQ (Ondo)",
				"alphaId": "ALPHA_746",
			},
		},
	}
	got := resolveAlphaSymbol(item, "QQQON", "USDT", docs)
	if got != "ALPHA_746USDT" {
		t.Fatalf("alpha symbol = %s", got)
	}
}

func TestResolveAlphaSymbolDoesNotFuzzyMatchShortName(t *testing.T) {
	item := config.AlphaItem{ID: "diaon", Name: "DIA", Symbol: "DIAONUSDT"}
	docs := []any{
		[]any{
			map[string]any{
				"symbol":  "NVDAon",
				"name":    "NVIDIA (Ondo)",
				"alphaId": "ALPHA_692",
			},
		},
	}
	got := resolveAlphaSymbol(item, "DIAON", "USDT", docs)
	if got != "DIAONUSDT" {
		t.Fatalf("alpha symbol = %s", got)
	}
}

func TestParseTickerMessage(t *testing.T) {
	raw := []byte(`{"stream":"alpha_175usdt@ticker","data":{"e":"24hrTicker","E":1773109631569,"s":"ALPHA_175USDT","P":"1.25","c":"12.34","v":"456.7"}}`)
	tick, ok := parseTickerMessage(raw)
	if !ok {
		t.Fatal("ticker not parsed")
	}
	if tick.Symbol != "ALPHA_175USDT" || tick.Price != 12.34 || tick.Change24hPct != 1.25 || tick.Volume != 456.7 {
		t.Fatalf("ticker = %+v", tick)
	}
}

func TestParseReferenceTickersUsesTokenListPrice(t *testing.T) {
	now := time.Unix(1773109631, 0).UTC()
	items := []ResolvedItem{{
		Item:        config.AlphaItem{ID: "aaplon", Name: "AAPL", Symbol: "AAPLONUSDT"},
		Category:    "stock",
		BaseSymbol:  "AAPLON",
		AlphaSymbol: "ALPHA_741USDT",
	}}
	rows := []any{
		map[string]any{
			"symbol":           "AAPLon",
			"alphaId":          "ALPHA_741",
			"price":            "309.273205787433351733",
			"percentChange24h": "0.65",
			"volume24h":        "13468706507.537016081318",
		},
	}
	got := parseReferenceTickers(rows, items, "USDT", now)
	tick, ok := got["AAPLON"]
	if !ok {
		t.Fatalf("reference quote missing: %+v", got)
	}
	if tick.Symbol != "ALPHA_741USDT" || tick.Price != 309.273205787433351733 || tick.Change24hPct != 0.65 || tick.Volume != 13468706507.537016081318 {
		t.Fatalf("reference quote = %+v", tick)
	}
	if !tick.UpdatedAt.Equal(now) {
		t.Fatalf("updatedAt = %s", tick.UpdatedAt)
	}
}

func TestParseKlineRows(t *testing.T) {
	candles := parseKlineRows([]any{
		[]any{"1752642000000", "10", "12", "9", "11", "123.4", "1752645599999"},
	})
	if len(candles) != 1 {
		t.Fatalf("candles len = %d", len(candles))
	}
	if candles[0].Time != 1752642000 || candles[0].Open != 10 || candles[0].Close != 11 || candles[0].Volume != 123.4 {
		t.Fatalf("candles = %+v", candles)
	}
}
