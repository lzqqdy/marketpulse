package portfolio

import "testing"

func TestNormalizeCryptoSymbol(t *testing.T) {
	if got := NormalizeCryptoSymbol("btcusdt"); got != "BTC" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeCryptoSymbol("ETH"); got != "ETH" {
		t.Fatalf("got %s", got)
	}
	if got := NormalizeCryptoSymbol("USDT"); got != "USDT" {
		t.Fatalf("usdt got %s", got)
	}
}

func TestWindowPnL(t *testing.T) {
	w := WindowPnL(110, 100, true)
	if w == nil || w.PnlCny != 10 || w.PnlPct == nil || *w.PnlPct != 10 {
		t.Fatalf("unexpected %+v", w)
	}
	if WindowPnL(100, 0, false) != nil {
		t.Fatal("expected nil without baseline")
	}
}

func TestAllTimePnL_zeroPrincipal(t *testing.T) {
	w := AllTimePnL(1000, 0)
	if w == nil || w.PnlCny != 1000 || w.PnlPct != nil {
		t.Fatalf("expected pnl without pct, got %+v", w)
	}
}

type stubResolver struct {
	crypto map[string]PricePoint
	alpha  map[string]PricePoint
	usdt   float64
	usd    float64
	ok     bool
}

func (s stubResolver) CryptoPrice(symbol string) PricePoint { return s.crypto[symbol] }
func (s stubResolver) AlphaPrice(id string) PricePoint      { return s.alpha[id] }
func (s stubResolver) UsdtCny() (float64, float64, bool)    { return s.usdt, s.usd, s.ok }

func TestValueHoldings(t *testing.T) {
	r := stubResolver{
		crypto: map[string]PricePoint{
			"BTC": {PriceUsdt: 100, ChangeDayPct: -1, OK: true},
			"USDT": {PriceUsdt: 1, OK: true},
		},
		alpha: map[string]PricePoint{
			"nvda": {PriceUsdt: 10, ChangeDayPct: 2, OK: true},
		},
		usdt: 7,
		usd:  7,
		ok:   true,
	}
	v := ValueHoldings(r, []Holding{
		{AssetType: AssetTypeCrypto, Symbol: "BTC", Quantity: 0.5},
		{AssetType: AssetTypeAlpha, Symbol: "nvda", Quantity: 2},
		{AssetType: AssetTypeCrypto, Symbol: "USDT", Quantity: 10},
	}, 6.5)
	if v.TotalUsdt != 50+20+10 {
		t.Fatalf("total usdt = %v", v.TotalUsdt)
	}
	if RoundMoney(v.TotalCny) != RoundMoney(80*7) {
		t.Fatalf("total cny = %v", v.TotalCny)
	}
	if len(v.Missing) != 0 {
		t.Fatalf("missing = %v", v.Missing)
	}
}

func TestDailyRatesFromPrev(t *testing.T) {
	d, dr, tp, tr := DailyRatesFromPrev(110, 100, 50)
	if d != 10 || dr != 0.1 || tp != 60 || tr != 1.2 {
		t.Fatalf("%v %v %v %v", d, dr, tp, tr)
	}
}
