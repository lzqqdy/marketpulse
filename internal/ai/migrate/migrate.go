package migrate

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	platformmigrate "github.com/lzqqdy/marketpulse/internal/platform/migrate"
)

//go:embed 001_ai.sql
var aiSQL string

//go:embed 002_ai_usage.sql
var aiUsageSQL string

// Run applies AI-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 4, Name: "ai", Up: aiSQL},
		{Version: 5, Name: "ai_usage_daily", Up: aiUsageSQL},
	}); err != nil {
		return fmt.Errorf("ai migrate: %w", err)
	}
	return nil
}
