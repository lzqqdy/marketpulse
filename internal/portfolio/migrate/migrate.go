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

// Run applies portfolio-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 3, Name: "portfolio", Up: portfolioSQL},
	}); err != nil {
		return fmt.Errorf("portfolio migrate: %w", err)
	}
	return nil
}
