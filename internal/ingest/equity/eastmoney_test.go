package equity

import "testing"

func TestEastmoneyKlineDataCandles(t *testing.T) {
	data := eastmoneyKlineData{
		Klines: []string{
			"2026-05-15,49930.26,49526.17,49930.26,49503.57,588000000,0.00,0.85,-1.07,-537.29,0.00",
			"2026-05-18,4547.6,4537.5,4559.0,4483.5,27964,0.0,1.66,-0.53,-24.4,0.00",
		},
	}
	candles, err := data.candles()
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("len=%d", len(candles))
	}
	if candles[0].Open != 49930.26 || candles[1].Close != 4537.5 {
		t.Fatalf("candles=%+v", candles)
	}
}

func TestNormalizeEastmoneyKlineInterval(t *testing.T) {
	klt, err := normalizeEastmoneyKlineInterval("1w")
	if err != nil {
		t.Fatal(err)
	}
	if klt != "102" {
		t.Fatalf("klt=%s", klt)
	}
}
