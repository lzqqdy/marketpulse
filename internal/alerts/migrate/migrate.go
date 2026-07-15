package migrate

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	platformmigrate "github.com/lzqqdy/marketpulse/internal/platform/migrate"
)

//go:embed 001_alerts.sql
var alertsSQL string

// Run applies alerts-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 2, Name: "alerts", Up: alertsSQL},
	}); err != nil {
		return fmt.Errorf("alerts migrate: %w", err)
	}
	return nil
}
