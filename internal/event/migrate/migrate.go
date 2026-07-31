package migrate

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	platformmigrate "github.com/lzqqdy/marketpulse/internal/platform/migrate"
)

//go:embed 001_event.sql
var eventSQL string

// Run applies event-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 6, Name: "event", Up: eventSQL},
	}); err != nil {
		return fmt.Errorf("event migrate: %w", err)
	}
	return nil
}
