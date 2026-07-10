package marketcenter

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMarket(t *testing.T) {
	if _, err := NormalizeMarket("ab"); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeMarket("xx"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePercent(t *testing.T) {
	if got := parsePercent("+10.36%"); got != 10.36 {
		t.Fatalf("got %v", got)
	}
	if got := parsePercent("-1.00%"); got != -1.0 {
		t.Fatalf("got %v", got)
	}
}

func TestHeatmapTypeCode(t *testing.T) {
	if HeatmapTypeCode(MarketHK) != "HSHY" {
		t.Fatal("hk type")
	}
	if HeatmapTypeCode(MarketAB) != "HY" {
		t.Fatal("ab type")
	}
}

func TestParseMinuteTrend(t *testing.T) {
	got := parseMinuteTrend("1,2,3,4,5")
	if len(got) != 5 || got[0] != 1 {
		t.Fatalf("got %v", got)
	}
	raw := json.RawMessage(`{"priceinfo":[{"price":10},{"price":11},{"price":9}]}`)
	got = parseMinuteTrendRaw(raw)
	if len(got) != 3 || got[1] != 11 {
		t.Fatalf("raw got %v", got)
	}
}

func TestDownsampleTrend(t *testing.T) {
	in := make([]float64, 100)
	for i := range in {
		in[i] = float64(i)
	}
	got := downsampleTrend(in)
	if len(got) != maxTrendPoints {
		t.Fatalf("got len %d", len(got))
	}
	if got[0] != 0 || got[len(got)-1] != 99 {
		t.Fatalf("endpoints %v", got)
	}
}
