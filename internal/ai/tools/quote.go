package tools

import (
	"fmt"
	"math"
	"time"

	"github.com/lzqqdy/marketpulse/internal/ai/evidence"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

func (r *Registry) getQuote(args map[string]any) (string, error) {
	inst, err := r.resolveInstrument(asString(args["symbol"]), asString(args["assetClass"]))
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": err.Error()})
	}
	out := inst.fields("")
	out["ok"] = true
	var updatedAt, fetchedAt time.Time
	var price float64
	maxAge := marketdata.SpotQuoteMaxAge
	switch inst.Asset {
	case "index", "us_equity":
		q, ok := r.md.IndexQuote(inst.ID)
		if !ok {
			out["ok"], out["error"] = false, fmt.Sprintf("%s quote not found: %s", inst.Asset, inst.ID)
			return marshal(out)
		}
		out["id"], out["name"], out["source"] = q.ID, q.Name, q.Source
		out["price"], out["changePct"], out["stale"] = q.Price, q.ChangePct, q.Stale
		updatedAt, fetchedAt, price = q.UpdatedAt, q.FetchedAt, q.Price
		// Closed-market quotes may be old; fetchedAt verifies the latest available session.
		maxAge = marketdata.IndexObservationMaxAge(q.ID, r.now())
		if fetchedAt.IsZero() {
			fetchedAt = updatedAt
		}
		out["sessionNote"] = "latest_available_trading_session"
	case "us_contract":
		q, ok := r.md.AlphaQuote(inst.ID)
		if !ok {
			out["ok"], out["error"] = false, "US reference quote not found: "+inst.ID
			return marshal(out)
		}
		out = inst.fields(q.Source)
		out["ok"], out["id"], out["name"], out["source"] = true, q.ID, q.Name, q.Source
		out["price"], out["change24hPct"], out["volume"], out["category"] = q.Price, q.Change24hPct, q.Volume, q.Category
		if q.MarkPrice > 0 {
			out["markPrice"] = q.MarkPrice
		}
		if q.IndexPrice > 0 {
			out["indexPrice"] = q.IndexPrice
		}
		if q.FundingRate != 0 {
			out["fundingRate"] = q.FundingRate
		}
		out["limitations"] = []string{"参考工具并非股票现货；24h 涨跌与美股交易日涨跌不可混用"}
		if q.Source == "bitget" {
			out["productType"] = "USDT-FUTURES"
		}
		updatedAt, fetchedAt, price = q.UpdatedAt, q.UpdatedAt, q.Price
		maxAge = marketdata.ReferenceQuoteMaxAge
	default:
		q, ok := r.md.Quote(inst.ID)
		if !ok {
			out["ok"], out["error"] = false, "crypto quote not found: "+inst.ID
			return marshal(out)
		}
		out["source"] = "binance"
		out["priceUsdt"], out["priceCny"] = q.PriceUsdt, q.PriceCny
		out["change24hPct"], out["changeDayPct"] = q.Change24hPct, q.ChangeDayPct
		out["rank"], out["marketCapUsd"], out["volume24hUsd"] = q.Rank, q.MarketCapUsd, q.Volume24hUsd
		updatedAt, fetchedAt, price = q.UpdatedAt, q.UpdatedAt, q.PriceUsdt
	}
	out["updatedAt"], out["fetchedAt"] = updatedAt, fetchedAt
	out["expiresAt"] = fetchedAt.Add(maxAge)
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		out["ok"], out["status"], out["error"] = false, evidence.Unavailable, "valid quote price unavailable"
		delete(out, "price")
		delete(out, "priceUsdt")
		delete(out, "priceCny")
	} else if out["stale"] == true || updatedAt.IsZero() || updatedAt.After(r.now().Add(time.Minute)) || marketdata.IsStale(fetchedAt, r.now(), maxAge) {
		out["ok"], out["status"], out["stale"] = false, evidence.Stale, true
		out["error"] = "quote is stale or its data time is unknown; historical reference only"
	}
	return marshal(out)
}
