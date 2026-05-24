package binance

import (
	"encoding/json"
	"testing"
)

func Test_normalizeMiniTicker_direct(t *testing.T) {
	ev := miniTickerEvent{Symbol: "BTCUSDT", Close: "100.5", Open: "99.0", High: "101", Low: "98"}
	tick, ok := normalizeMiniTicker(ev)
	if !ok || tick.Symbol != "BTC" {
		t.Fatalf("direct: %+v ok=%v", tick, ok)
	}
}

func Test_parseMiniTickerMessage_combined(t *testing.T) {
	raw := []byte(`{"stream":"btcusdt@miniTicker","data":{"e":"24hrMiniTicker","E":123,"s":"BTCUSDT","c":"100.5","o":"99.0","h":"101","l":"98"}}`)

	var wrap combinedMessage
	if err := json.Unmarshal(raw, &wrap); err != nil {
		t.Fatal(err)
	}
	if len(wrap.Data) == 0 {
		t.Fatal("wrap.Data empty")
	}
	var ev miniTickerEvent
	if err := json.Unmarshal(wrap.Data, &ev); err != nil {
		t.Fatal(err)
	}

	tick, ok := parseMiniTickerMessage(raw)
	if !ok || tick.Symbol != "BTC" || tick.PriceUsdt != 100.5 {
		t.Fatalf("tick: %+v ok=%v ev=%+v", tick, ok, ev)
	}
	if tick.Change24hPct <= 0 {
		t.Fatalf("chg: %f", tick.Change24hPct)
	}
}
