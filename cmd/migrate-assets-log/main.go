// Command migrate-assets-log imports legacy assets_log / assets into MarketPulse portfolio tables.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lzqqdy/marketpulse/internal/portfolio"
)

func main() {
	legacyDSN := flag.String("legacy-dsn", "", "MySQL DSN for legacy DB (user:pass@tcp(host:3306)/dbname?parseTime=true)")
	mpDSN := flag.String("mp-dsn", "", "MySQL DSN for MarketPulse DB")
	uidMapPath := flag.String("uid-map", "", "uid map file: lines `old new` or JSON object")
	overwrite := flag.Bool("overwrite", false, "overwrite existing daily snapshots")
	dryRun := flag.Bool("dry-run", false, "parse and report without writing")
	withHoldings := flag.Bool("holdings", true, "also import legacy assets table into holdings")
	flag.Parse()

	if *legacyDSN == "" || *mpDSN == "" || *uidMapPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	raw, err := os.ReadFile(*uidMapPath)
	if err != nil {
		log.Fatalf("read uid map: %v", err)
	}
	uidMap, err := portfolio.ParseUIDMapYAML(string(raw))
	if err != nil {
		log.Fatalf("parse uid map: %v", err)
	}

	legacyDB, err := sql.Open("mysql", *legacyDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer legacyDB.Close()
	mpDB, err := sql.Open("mysql", *mpDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer mpDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	logs, err := loadAssetsLog(ctx, legacyDB)
	if err != nil {
		log.Fatalf("load assets_log: %v", err)
	}
	var assets []portfolio.LegacyAssetRow
	if *withHoldings {
		assets, err = loadAssets(ctx, legacyDB)
		if err != nil {
			log.Fatalf("load assets: %v", err)
		}
	}
	balances, err := loadBalances(ctx, legacyDB)
	if err != nil {
		log.Printf("warn: load balances skipped: %v", err)
		balances = map[int64]float64{}
	}

	report, err := portfolio.ImportLegacy(ctx, mpDB, logs, assets, balances, portfolio.LegacyImportOptions{
		UIDMap:         uidMap,
		Overwrite:      *overwrite,
		DryRun:         *dryRun,
		ImportHoldings: *withHoldings,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("dry_run=%v snapshots_inserted=%d skipped=%d unmapped_skipped=%d settings=%d holdings=%d errors=%d\n",
		*dryRun, report.SnapshotsInserted, report.SnapshotsSkipped, report.UnmappedSkipped, report.SettingsUpdated, report.HoldingsUpdated, len(report.Errors))
	for i, e := range report.Errors {
		if i >= 20 {
			fmt.Printf("... %d more errors\n", len(report.Errors)-20)
			break
		}
		fmt.Println("ERR:", e)
	}
}

func loadAssetsLog(ctx context.Context, db *sql.DB) ([]portfolio.LegacyAssetsLogRow, error) {
	rows, err := db.QueryContext(ctx, `
SELECT uid, date, total_value, total_value_cny, daily_profit, daily_profit_rate,
  total_profit, total_profit_rate, COALESCE(asset_detail,''), COALESCE(ctime,0)
FROM assets_log ORDER BY uid, date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []portfolio.LegacyAssetsLogRow
	for rows.Next() {
		var r portfolio.LegacyAssetsLogRow
		var date any
		if err := rows.Scan(&r.UID, &date, &r.TotalValue, &r.TotalValueCny, &r.DailyProfit, &r.DailyProfitRate,
			&r.TotalProfit, &r.TotalProfitRate, &r.AssetDetail, &r.Ctime); err != nil {
			return nil, err
		}
		r.Date = stringifyDate(date)
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadAssets(ctx context.Context, db *sql.DB) ([]portfolio.LegacyAssetRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT uid, coin, total_num, COALESCE(target_price,0) FROM assets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []portfolio.LegacyAssetRow
	for rows.Next() {
		var r portfolio.LegacyAssetRow
		if err := rows.Scan(&r.UID, &r.Coin, &r.TotalNum, &r.TargetPrice); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadBalances(ctx context.Context, db *sql.DB) (map[int64]float64, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(balance,0) FROM user`)
	if err != nil {
		// try users table name variants
		rows, err = db.QueryContext(ctx, `SELECT id, COALESCE(balance,0) FROM users`)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	out := map[int64]float64{}
	for rows.Next() {
		var id int64
		var bal float64
		if err := rows.Scan(&id, &bal); err != nil {
			return nil, err
		}
		out[id] = bal
	}
	return out, rows.Err()
}

func stringifyDate(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02")
	case []byte:
		s := string(t)
		if s == "1" {
			return "1"
		}
		if len(s) >= 10 {
			return s[:10]
		}
		return s
	case string:
		if t == "1" {
			return "1"
		}
		if len(t) >= 10 && strings.Contains(t, "-") {
			return t[:10]
		}
		return t
	default:
		return fmt.Sprint(v)
	}
}
