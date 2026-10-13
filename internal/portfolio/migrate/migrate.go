package migrate

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	platformmigrate "github.com/lzqqdy/marketpulse/internal/platform/migrate"
)

//go:embed 001_portfolio.sql
var portfolioSQL string

//go:embed 002_snapshot_valuation.sql
var snapshotValuationSQL string

// Run applies portfolio-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 3, Name: "portfolio", Up: portfolioSQL},
		{Version: 7, Name: "portfolio_snapshot_valuation", Up: snapshotValuationSQL},
	}); err != nil {
		return fmt.Errorf("portfolio migrate: %w", err)
	}
	return nil
}
