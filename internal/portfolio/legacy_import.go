package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// LegacyImportOptions controls assets_log import behaviour.
type LegacyImportOptions struct {
	UIDMap     map[int64]int64 // old uid -> new user id
	Overwrite  bool
	DryRun     bool
	ImportHoldings bool
}

// LegacyAssetsLogRow mirrors old assets_log.
type LegacyAssetsLogRow struct {
	UID             int64
	Date            string
	TotalValue      float64
	TotalValueCny   float64
	DailyProfit     float64
	DailyProfitRate float64
	TotalProfit     float64
	TotalProfitRate float64
	AssetDetail     string
	Ctime           int64
}

// LegacyAssetRow mirrors old assets.
type LegacyAssetRow struct {
	UID         int64
	Coin        string
	TotalNum    float64
	TargetPrice float64
}

// ImportReport summarizes a legacy import run.
type ImportReport struct {
	SnapshotsInserted int
	SnapshotsSkipped  int
	UnmappedSkipped   int // rows ignored because old uid not in UIDMap
	SettingsUpdated   int
	HoldingsUpdated   int
	Errors            []string
}

// ImportLegacy runs an import using an already-scanned row set against MarketPulse DB.
func ImportLegacy(ctx context.Context, db *sql.DB, logs []LegacyAssetsLogRow, assets []LegacyAssetRow, balances map[int64]float64, opt LegacyImportOptions) (ImportReport, error) {
	if len(opt.UIDMap) == 0 {
		return ImportReport{}, fmt.Errorf("uid map is required")
	}
	repo := newRepository(db)
	var report ImportReport

	for _, row := range logs {
		newUID, ok := opt.UIDMap[row.UID]
		if !ok {
			// Only mapped uids are imported; others are intentional skips, not errors.
			report.UnmappedSkipped++
			continue
		}
		date := strings.TrimSpace(row.Date)
		if date == "1" {
			principal := row.TotalValueCny
			if bal, ok := balances[row.UID]; ok && bal > 0 {
				principal = bal
			}
			if opt.DryRun {
				report.SettingsUpdated++
				continue
			}
			if _, err := repo.UpsertSettings(ctx, newUID, RoundMoney(principal), nil); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			report.SettingsUpdated++
			continue
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("bad date %q uid=%d", date, row.UID))
			continue
		}
		detail := normalizeLegacyDetail(row.AssetDetail)
		rate := row.DailyProfitRate
		totalRate := row.TotalProfitRate
		created := time.Now()
		if row.Ctime > 0 {
			created = time.Unix(row.Ctime, 0)
		}
		snap := Snapshot{
			UserID:          newUID,
			Date:            date,
			Kind:            SnapshotKindDaily,
			TotalValue:      row.TotalValue,
			TotalValueCny:   row.TotalValueCny,
			DailyProfit:     row.DailyProfit,
			DailyProfitRate: rate,
			TotalProfit:     row.TotalProfit,
			TotalProfitRate: totalRate,
			AssetDetail:     detail,
			Source:          SourceLegacy,
			CreatedAt:       created,
		}
		if opt.DryRun {
			report.SnapshotsInserted++
			continue
		}
		if opt.Overwrite {
			if err := repo.UpsertDailySnapshot(ctx, snap); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			report.SnapshotsInserted++
			continue
		}
		inserted, err := repo.InsertSnapshotSkip(ctx, snap)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		if inserted {
			report.SnapshotsInserted++
		} else {
			report.SnapshotsSkipped++
		}
	}

	if opt.ImportHoldings && len(assets) > 0 {
		byUser := map[int64][]Holding{}
		for _, a := range assets {
			newUID, ok := opt.UIDMap[a.UID]
			if !ok {
				continue
			}
			sym := NormalizeCryptoSymbol(a.Coin)
			if sym == "" || a.TotalNum <= 0 {
				continue
			}
			h := Holding{UserID: newUID, AssetType: AssetTypeCrypto, Symbol: sym, Quantity: a.TotalNum}
			if a.TargetPrice > 0 {
				tp := a.TargetPrice
				h.TargetPrice = &tp
			}
			byUser[newUID] = append(byUser[newUID], h)
		}
		for uid, hs := range byUser {
			if opt.DryRun {
				report.HoldingsUpdated++
				continue
			}
			if err := repo.ReplaceHoldings(ctx, uid, hs); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			report.HoldingsUpdated++
		}
	}

	// balances without date=1
	for oldUID, bal := range balances {
		newUID, ok := opt.UIDMap[oldUID]
		if !ok || bal <= 0 {
			continue
		}
		if opt.DryRun {
			continue
		}
		cur, err := repo.GetSettings(ctx, newUID)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		if cur.PrincipalCny > 0 {
			continue
		}
		if _, err := repo.UpsertSettings(ctx, newUID, RoundMoney(bal), nil); err != nil {
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		report.SettingsUpdated++
	}

	return report, nil
}

func normalizeLegacyDetail(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "[]"
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return raw
	}
	out := make([]AssetDetailRow, 0, len(arr))
	for _, m := range arr {
		coin, _ := m["coin"].(string)
		if coin == "" {
			if s, ok := m["symbol"].(string); ok {
				coin = s
			}
		}
		qty := asFloat(m["total_num"])
		if qty == 0 {
			qty = asFloat(m["quantity"])
		}
		price := asFloat(m["price"])
		if price == 0 {
			price = asFloat(m["price_usdt"])
		}
		vu := asFloat(m["value_usdt"])
		if vu == 0 && price > 0 {
			vu = qty * price
		}
		vc := asFloat(m["value_cny"])
		out = append(out, AssetDetailRow{
			AssetType: AssetTypeCrypto,
			Symbol:    NormalizeCryptoSymbol(coin),
			Quantity:  qty,
			PriceUsdt: price,
			ValueUsdt: vu,
			ValueCny:  vc,
			Raw:       m,
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return string(b)
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

// ParseUIDMapYAML parses simple "old: new" lines or JSON object.
func ParseUIDMapYAML(raw string) (map[int64]int64, error) {
	raw = strings.TrimSpace(raw)
	out := map[int64]int64{}
	if raw == "" {
		return out, fmt.Errorf("empty uid map")
	}
	if strings.HasPrefix(raw, "{") {
		tmp := map[string]int64{}
		if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
			return nil, err
		}
		for k, v := range tmp {
			old, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			out[old] = v
		}
		return out, nil
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ReplaceAll(line, ":", " ")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		old, err1 := strconv.ParseInt(fields[0], 10, 64)
		neu, err2 := strconv.ParseInt(fields[1], 10, 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("bad uid map line: %s", line)
		}
		out[old] = neu
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("uid map empty after parse")
	}
	return out, nil
}
