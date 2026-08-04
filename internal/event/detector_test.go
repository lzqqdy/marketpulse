package event

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

func TestDetectPrice_Threshold(t *testing.T) {
	now := time.Now().UTC()
	candles := []binance.Candle{
		{Close: 100},
		{Close: 96}, // -4%
	}
	cfg := config.EventPriceConfig{"15m": {ThresholdPct: 3}}
	sigs := DetectPrice("BTC", candles, cfg, now)
	if len(sigs) != 1 || sigs[0].SignalType != SignalPriceDrop {
		t.Fatalf("expected PRICE_DROP, got %+v", sigs)
	}
	cfg2 := config.EventPriceConfig{"15m": {ThresholdPct: 5}}
	if got := DetectPrice("BTC", candles, cfg2, now); len(got) != 0 {
		t.Fatalf("expected no signal under higher threshold, got %+v", got)
	}
}

func TestDetectPrice_InsufficientCandles(t *testing.T) {
	cfg := config.EventPriceConfig{"15m": {ThresholdPct: 1}}
	if got := DetectPrice("BTC", []binance.Candle{{Close: 1}}, cfg, time.Now()); len(got) != 0 {
		t.Fatalf("expected skip, got %+v", got)
	}
}

func TestDetectVolume_Ratio(t *testing.T) {
	candles := make([]binance.Candle, 0, 22)
	for i := 0; i < 20; i++ {
		candles = append(candles, binance.Candle{Volume: 100})
	}
	candles = append(candles, binance.Candle{Volume: 100}) // lookback end
	candles = append(candles, binance.Candle{Volume: 400}) // current 4x
	cfg := config.EventVolumeConfig{LookbackBars: 20, RatioMedium: 2}
	sigs := DetectVolume("BTC", "15m", candles, cfg, time.Now())
	if len(sigs) != 1 || sigs[0].Ratio < 3 {
		t.Fatalf("expected volume spike, got %+v", sigs)
	}
}

func TestAggregate_FlashSelloff(t *testing.T) {
	now := time.Now().UTC()
	sigs := []EventSignal{
		{ID: "1", SignalType: SignalPriceDrop, Symbol: "BTC", Window: "15m", ChangePct: -4, Direction: DirectionDown, Timestamp: now},
		{ID: "2", SignalType: SignalVolumeSpike, Symbol: "BTC", Window: "15m", Ratio: 4, Timestamp: now},
		{ID: "3", SignalType: SignalLiquidationSpike, Symbol: SymbolMarketWide, Window: "1h", Ratio: 9, Timestamp: now},
	}
	res := Aggregate(AggregateInput{Now: now, Signals: sigs, AggregateWindow: 10 * time.Minute})
	if len(res.Created) != 1 {
		t.Fatalf("expected 1 event, got %d", len(res.Created))
	}
	if res.Created[0].SubType != SubFlashSelloff {
		t.Fatalf("expected FLASH_SELLOFF, got %s", res.Created[0].SubType)
	}
	if len(res.Created[0].Signals) < 3 {
		t.Fatalf("expected 3 signals attached, got %d", len(res.Created[0].Signals))
	}
}

func TestAggregate_PriceOnly(t *testing.T) {
	now := time.Now().UTC()
	sigs := []EventSignal{
		{ID: "1", SignalType: SignalPriceDrop, Symbol: "ETH", Window: "15m", ChangePct: -3.5, Direction: DirectionDown, Timestamp: now},
	}
	res := Aggregate(AggregateInput{Now: now, Signals: sigs})
	if len(res.Created) != 1 || res.Created[0].SubType != SubDrop {
		t.Fatalf("expected price DROP event, got %+v", res.Created)
	}
}

func TestAggregate_MergeOpen(t *testing.T) {
	now := time.Now().UTC()
	open := []MarketEvent{{
		ID:         "evt_open",
		Type:       TypePriceAnomaly,
		SubType:    SubDrop,
		Status:     StatusActive,
		StartTime:  now.Add(-2 * time.Minute),
		Symbols:    []string{"BTC"},
		TemplateID: "price_only",
		Signals: []EventSignal{
			{ID: "old", SignalType: SignalPriceDrop, Symbol: "BTC", Window: "15m", ChangePct: -3},
		},
	}}
	sigs := []EventSignal{
		{ID: "n1", SignalType: SignalPriceDrop, Symbol: "BTC", Window: "15m", ChangePct: -4},
		{ID: "n2", SignalType: SignalVolumeSpike, Symbol: "BTC", Window: "15m", Ratio: 3.5},
	}
	res := Aggregate(AggregateInput{Now: now, Signals: sigs, OpenEvents: open})
	if len(res.Created) != 0 {
		t.Fatalf("expected merge not create, got created=%d", len(res.Created))
	}
	if len(res.Updated) != 1 {
		t.Fatalf("expected 1 update, got %d", len(res.Updated))
	}
	if res.Updated[0].SubType != SubVolumeMove && res.Updated[0].SubType != SubFlashSelloff {
		// price+strong volume => volume_move
		if res.Updated[0].SubType != SubVolumeMove {
			t.Fatalf("expected volume_move upgrade, got %s", res.Updated[0].SubType)
		}
	}
}

func TestAggregate_WeakVolumeStaysPriceOnly(t *testing.T) {
	now := time.Now().UTC()
	sigs := []EventSignal{
		{ID: "1", SignalType: SignalPriceSpike, Symbol: "ETH", Window: "15m", ChangePct: 3.2, Direction: DirectionUp, Timestamp: now},
		{ID: "2", SignalType: SignalVolumeSpike, Symbol: "ETH", Window: "15m", Ratio: 2.2, Timestamp: now},
	}
	res := Aggregate(AggregateInput{Now: now, Signals: sigs})
	if len(res.Created) != 1 || res.Created[0].SubType != SubSpike {
		t.Fatalf("expected price_only SPIKE (weak volume), got %+v", res.Created)
	}
}

func TestAggregate_AlwaysMergeOpenIgnoringAge(t *testing.T) {
	now := time.Now().UTC()
	open := []MarketEvent{{
		ID:         "evt_old",
		Type:       TypePriceAnomaly,
		SubType:    SubDrop,
		Status:     StatusDeescalating,
		StartTime:  now.Add(-3 * time.Hour),
		Symbols:    []string{"ETH"},
		TemplateID: "price_only",
		Signals:    []EventSignal{{ID: "old", SignalType: SignalPriceDrop, Symbol: "ETH", Window: "15m", ChangePct: -3}},
	}}
	sigs := []EventSignal{
		{ID: "n1", SignalType: SignalPriceDrop, Symbol: "ETH", Window: "15m", ChangePct: -4},
	}
	res := Aggregate(AggregateInput{Now: now, Signals: sigs, OpenEvents: open, AggregateWindow: time.Minute})
	if len(res.Created) != 0 || len(res.Updated) != 1 {
		t.Fatalf("expected merge aged open event, created=%d updated=%d", len(res.Created), len(res.Updated))
	}
}
