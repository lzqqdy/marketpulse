package portfolio

import (
	"math"
	"strings"
)

// PricePoint is a resolved mark price in USDT terms.
type PricePoint struct {
	PriceUsdt    float64
	ChangeDayPct float64
	OK           bool
}

// PriceResolver looks up live prices without talking to exchanges.
type PriceResolver interface {
	CryptoPrice(symbol string) PricePoint
	AlphaPrice(id string) PricePoint
	UsdtCny() (rate float64, usdCny float64, ok bool)
}

// NormalizeCryptoSymbol turns BTCUSDT / btc into BTC; keeps USDT as USDT.
func NormalizeCryptoSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" || s == "USDT" {
		return s
	}
	return strings.TrimSuffix(s, "USDT")
}

// NormalizeAlphaSymbol lowercases alpha ids.
func NormalizeAlphaSymbol(symbol string) string {
	return strings.ToLower(strings.TrimSpace(symbol))
}

// NormalizeAssetType validates and returns canonical asset type.
func NormalizeAssetType(t string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case AssetTypeCrypto, "spot":
		return AssetTypeCrypto, true
	case AssetTypeAlpha:
		return AssetTypeAlpha, true
	default:
		return "", false
	}
}

// ResolvePrice resolves one holding mark.
func ResolvePrice(r PriceResolver, assetType, symbol string) PricePoint {
	switch assetType {
	case AssetTypeCrypto:
		sym := NormalizeCryptoSymbol(symbol)
		if sym == "USDT" {
			return PricePoint{PriceUsdt: 1, OK: true}
		}
		return r.CryptoPrice(sym)
	case AssetTypeAlpha:
		return r.AlphaPrice(NormalizeAlphaSymbol(symbol))
	default:
		return PricePoint{}
	}
}

// Valuation aggregates holdings into totals.
type Valuation struct {
	Holdings       []HoldingView
	TotalUsdt      float64
	TotalCny       float64
	UsdtCny        float64
	UsdtPremiumPct float64
	RateFallback   bool
	Missing        []string
	Details        []AssetDetailRow
}

// ValueHoldings values a list of holdings with the given rate fallback.
func ValueHoldings(r PriceResolver, holdings []Holding, defaultUsdtCny float64) Valuation {
	usdtCny, usdCny, rateOK := r.UsdtCny()
	rateFallback := false
	if !rateOK || usdtCny <= 0 {
		usdtCny = defaultUsdtCny
		rateFallback = true
	}
	premium := 0.0
	if usdCny > 0 && usdtCny > 0 {
		premium = (usdtCny - usdCny) / usdCny * 100
	}

	out := Valuation{
		Holdings:       make([]HoldingView, 0, len(holdings)),
		UsdtCny:        usdtCny,
		UsdtPremiumPct: premium,
		RateFallback:   rateFallback,
		Details:        make([]AssetDetailRow, 0, len(holdings)),
	}

	for _, h := range holdings {
		pp := ResolvePrice(r, h.AssetType, h.Symbol)
		view := HoldingView{
			AssetType: h.AssetType,
			Symbol:    h.Symbol,
			Quantity:  h.Quantity,
		}
		if !pp.OK || pp.PriceUsdt < 0 || (pp.PriceUsdt == 0 && NormalizeCryptoSymbol(h.Symbol) != "USDT") {
			view.Missing = true
			out.Missing = append(out.Missing, displaySymbol(h.AssetType, h.Symbol))
			out.Holdings = append(out.Holdings, view)
			continue
		}
		view.PriceUsdt = pp.PriceUsdt
		view.ValueUsdt = h.Quantity * pp.PriceUsdt
		view.ValueCny = view.ValueUsdt * usdtCny
		view.ChangeCny = view.ValueCny * pp.ChangeDayPct / 100
		out.TotalUsdt += view.ValueUsdt
		out.TotalCny += view.ValueCny
		out.Holdings = append(out.Holdings, view)
		out.Details = append(out.Details, AssetDetailRow{
			AssetType: h.AssetType,
			Symbol:    h.Symbol,
			Quantity:  h.Quantity,
			PriceUsdt: view.PriceUsdt,
			ValueUsdt: view.ValueUsdt,
			ValueCny:  view.ValueCny,
		})
	}
	return out
}

func displaySymbol(assetType, symbol string) string {
	if assetType == AssetTypeCrypto {
		return NormalizeCryptoSymbol(symbol)
	}
	return NormalizeAlphaSymbol(symbol)
}

// WindowPnL computes CNY delta vs a baseline snapshot total.
func WindowPnL(totalCny float64, baselineCny float64, hasBaseline bool) *PnLWindow {
	if !hasBaseline {
		return nil
	}
	pnl := totalCny - baselineCny
	w := &PnLWindow{PnlCny: pnl}
	if baselineCny != 0 {
		pct := pnl / baselineCny * 100
		w.PnlPct = &pct
	}
	return w
}

// AllTimePnL is current CNY minus principal.
func AllTimePnL(totalCny, principalCny float64) *PnLWindow {
	pnl := totalCny - principalCny
	w := &PnLWindow{PnlCny: pnl}
	if principalCny > 0 {
		pct := pnl / principalCny * 100
		w.PnlPct = &pct
	}
	return w
}

// DailyRatesFromPrev returns decimal rates for snapshot storage.
func DailyRatesFromPrev(totalCny, prevCny, principalCny float64) (dailyProfit, dailyRate, totalProfit, totalRate float64) {
	dailyProfit = totalCny - prevCny
	if prevCny != 0 {
		dailyRate = dailyProfit / prevCny
	}
	totalProfit = totalCny - principalCny
	if principalCny > 0 {
		totalRate = totalProfit / principalCny
	}
	return
}

// RoundMoney rounds to 2 decimals for CNY display storage.
func RoundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
