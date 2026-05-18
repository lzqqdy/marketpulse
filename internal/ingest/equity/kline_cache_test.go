package equity

import (
	"testing"

	"github.com/lzqqdy/marketpulse/internal/binance"
)

func TestMergeCandlesOverwritesAndAppends(t *testing.T) {
	oldRows := []binance.Candle{
		{Time: 100, Open: 1, High: 2, Low: 1, Close: 2},
		{Time: 200, Open: 2, High: 3, Low: 2, Close: 3},
		{Time: 300, Open: 3, High: 4, Low: 3, Close: 4},
	}
	newRows := []binance.Candle{
		{Time: 200, Open: 20, High: 30, Low: 20, Close: 30},
		{Time: 400, Open: 4, High: 5, Low: 4, Close: 5},
	}

	got := mergeCandles(oldRows, newRows)
	if len(got) != 4 {
		t.Fatalf("len = %d, rows = %+v", len(got), got)
	}
	if got[0].Time != 100 || got[1].Time != 200 || got[2].Time != 300 || got[3].Time != 400 {
		t.Fatalf("unexpected order: %+v", got)
	}
	if got[1].Open != 20 || got[1].Close != 30 {
		t.Fatalf("expected overwritten candle at 200, got %+v", got[1])
	}
}

func TestTrimCandlesKeepsLatest(t *testing.T) {
	rows := []binance.Candle{{Time: 100}, {Time: 200}, {Time: 300}}
	got := trimCandles(rows, 2)
	if len(got) != 2 || got[0].Time != 200 || got[1].Time != 300 {
		t.Fatalf("trim = %+v", got)
	}
}
