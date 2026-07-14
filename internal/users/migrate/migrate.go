package migrate

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	platformmigrate "github.com/lzqqdy/marketpulse/internal/platform/migrate"
)

//go:embed 001_users.sql
var usersSQL string

// Run applies users-module schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if err := platformmigrate.Run(ctx, db, []platformmigrate.Migration{
		{Version: 1, Name: "users", Up: usersSQL},
	}); err != nil {
		return fmt.Errorf("users migrate: %w", err)
	}
	return nil
}
