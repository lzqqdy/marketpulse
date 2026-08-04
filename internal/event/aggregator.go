package event

import (
	"fmt"
	"strings"
	"time"
)

// AggregateInput is one evaluation round of fresh signals plus open events.
type AggregateInput struct {
	Now             time.Time
	Signals         []EventSignal
	OpenEvents      []MarketEvent
	AggregateWindow time.Duration
}

// AggregateResult holds creates and updates for persistence.
type AggregateResult struct {
	Created    []MarketEvent
	Updated    []MarketEvent
	NewSignals map[string][]EventSignal // eventID -> newly attached signals
}

// Aggregate merges signals into templates (flash_selloff / volume_move / price_only).
func Aggregate(in AggregateInput) AggregateResult {
	now := in.Now.UTC()
	res := AggregateResult{NewSignals: map[string][]EventSignal{}}

	bySymbol := map[string][]EventSignal{}
	var liq []EventSignal
	for _, s := range in.Signals {
		if s.SignalType == SignalLiquidationSpike {
			liq = append(liq, s)
			continue
		}
		sym := strings.ToUpper(s.Symbol)
		bySymbol[sym] = append(bySymbol[sym], s)
	}

	openBySymbol := map[string]*MarketEvent{}
	for i := range in.OpenEvents {
		ev := &in.OpenEvents[i]
		if ev.TemplateID == "" {
			ev.TemplateID = inferTemplateID(ev)
		}
		sym := primarySymbol(ev)
		// Prefer keeping the highest-tier open event per symbol.
		if prev, ok := openBySymbol[sym]; ok {
			if templateRank(ev.TemplateID) <= templateRank(prev.TemplateID) {
				continue
			}
		}
		openBySymbol[sym] = ev
	}

	for sym, sigs := range bySymbol {
		bundle := append([]EventSignal(nil), sigs...)
		bundle = append(bundle, liq...)
		tmpl, title, typ, sub := matchTemplate(bundle)
		if tmpl == "" {
			continue
		}
		// Hard rule: at most one OPEN event per symbol — always merge while open.
		if existing, ok := openBySymbol[sym]; ok {
			attached := filterNewSignals(existing, sigs)
			if len(liq) > 0 {
				attached = append(attached, filterNewSignals(existing, liq)...)
			}
			for i := range attached {
				attached[i].EventID = existing.ID
			}
			if len(attached) > 0 {
				existing.Signals = append(existing.Signals, attached...)
			}
			existing.Symbols = mergeSymbols(existing.Symbols, sym)
			// Re-match template against full signal set so events can upgrade.
			if nt, ntitle, ntyp, nsub := matchTemplate(existing.Signals); nt != "" {
				existing.TemplateID = nt
				existing.Title = ntitle
				existing.Type = ntyp
				existing.SubType = nsub
			} else {
				existing.TemplateID = tmpl
				existing.Title = title
				existing.Type = typ
				existing.SubType = sub
			}
			score, sev := ScoreFromSignals(existing.Signals)
			existing.Severity = sev
			ApplyLifecycle(existing, score, now)
			existing.Description = buildDescription(existing.Signals)
			res.Updated = append(res.Updated, *existing)
			if len(attached) > 0 {
				res.NewSignals[existing.ID] = attached
			}
			continue
		}

		all := append([]EventSignal(nil), sigs...)
		all = append(all, liq...)
		id := newEventID()
		for i := range all {
			all[i].EventID = id
			if all[i].ID == "" {
				all[i].ID = newSignalID()
			}
		}
		score, sev := ScoreFromSignals(all)
		ev := MarketEvent{
			ID:          id,
			Type:        typ,
			SubType:     sub,
			Title:       title,
			Description: buildDescription(all),
			Severity:    sev,
			Score:       score,
			PeakScore:   score,
			Status:      StatusDetected,
			StartTime:   now,
			Symbols:     []string{sym},
			Markets:     []string{MarketCrypto},
			Signals:     all,
			Context:     map[string]any{},
			CreatedAt:   now,
			UpdatedAt:   now,
			TemplateID:  tmpl,
		}
		ApplyLifecycle(&ev, score, now)
		res.Created = append(res.Created, ev)
		res.NewSignals[id] = all
		openBySymbol[sym] = &res.Created[len(res.Created)-1]
	}

	return res
}

func templateRank(tmpl string) int {
	switch tmpl {
	case "flash_selloff":
		return 3
	case "volume_move":
		return 2
	case "price_only":
		return 1
	default:
		return 0
	}
}

func matchTemplate(sigs []EventSignal) (templateID, title, typ, sub string) {
	hasPriceDrop := false
	hasPriceSpike := false
	hasVol := false
	hasStrongVol := false
	hasLiq := false
	for _, s := range sigs {
		switch s.SignalType {
		case SignalPriceDrop:
			hasPriceDrop = true
		case SignalPriceSpike:
			hasPriceSpike = true
		case SignalVolumeSpike:
			hasVol = true
			// volume_move needs meaningful size; lone ~2x+tiny price was spammy.
			if s.Ratio >= 3 {
				hasStrongVol = true
			}
		case SignalLiquidationSpike:
			hasLiq = true
		}
	}
	if hasPriceDrop && hasVol && hasLiq {
		return "flash_selloff", "加密资产快速走弱", TypeCryptoMove, SubFlashSelloff
	}
	if (hasPriceDrop || hasPriceSpike) && hasStrongVol {
		return "volume_move", "加密成交量放大异动", TypeCryptoMove, SubVolumeMove
	}
	if hasPriceDrop {
		return "price_only", "加密价格快速下跌", TypePriceAnomaly, SubDrop
	}
	if hasPriceSpike {
		return "price_only", "加密价格快速上涨", TypePriceAnomaly, SubSpike
	}
	return "", "", "", ""
}

func primarySymbol(ev *MarketEvent) string {
	for _, s := range ev.Symbols {
		if s != "" && s != SymbolMarketWide {
			return strings.ToUpper(s)
		}
	}
	return SymbolMarketWide
}

func inferTemplateID(ev *MarketEvent) string {
	switch ev.SubType {
	case SubFlashSelloff:
		return "flash_selloff"
	case SubVolumeMove:
		return "volume_move"
	default:
		return "price_only"
	}
}

func filterNewSignals(ev *MarketEvent, candidates []EventSignal) []EventSignal {
	seen := map[string]struct{}{}
	for _, s := range ev.Signals {
		seen[s.SignalType+"|"+s.Symbol+"|"+s.Window] = struct{}{}
	}
	out := make([]EventSignal, 0)
	for _, c := range candidates {
		key := c.SignalType + "|" + c.Symbol + "|" + c.Window
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, c)
		seen[key] = struct{}{}
	}
	return out
}

func mergeSymbols(existing []string, sym string) []string {
	sym = strings.ToUpper(sym)
	for _, s := range existing {
		if strings.EqualFold(s, sym) {
			return existing
		}
	}
	return append(append([]string{}, existing...), sym)
}

func buildDescription(sigs []EventSignal) string {
	parts := make([]string, 0, len(sigs))
	for _, s := range sigs {
		switch s.SignalType {
		case SignalPriceDrop, SignalPriceSpike:
			parts = append(parts, s.Symbol+" "+s.Window+" "+formatPct(s.ChangePct))
		case SignalVolumeSpike:
			parts = append(parts, s.Symbol+" 量能 "+formatRatio(s.Ratio))
		case SignalLiquidationSpike:
			parts = append(parts, "爆仓 "+formatRatio(s.Ratio))
		case SignalVolatilitySpike:
			parts = append(parts, s.Symbol+" 波动 "+formatRatio(s.Ratio))
		}
	}
	if len(parts) == 0 {
		return "市场异动"
	}
	return strings.Join(parts, "；")
}

func formatPct(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

func formatRatio(v float64) string {
	return fmt.Sprintf("%.1fx", v)
}
