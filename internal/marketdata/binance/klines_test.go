package binance

import "testing"

func TestNormalizeInterval(t *testing.T) {
	_, err := NormalizeInterval("2h")
	if err == nil {
		t.Fatal("expected error")
	}
	v, err := NormalizeInterval("1H")
	if err != nil || v != "1h" {
		t.Fatalf("got %s %v", v, err)
	}
}

func TestSymbolUSDT(t *testing.T) {
	if SymbolUSDT("btc") != "BTCUSDT" {
		t.Fatal()
	}
}
