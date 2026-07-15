// Package migrate runs numbered SQL migrations stored in schema_migrations.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Migration is one versioned SQL statement batch (MySQL dialect).
type Migration struct {
	Version int
	Name    string
	Up      string
}

// Run applies pending migrations in ascending Version order inside transactions.
func Run(ctx context.Context, db *sql.DB, migrations []Migration) error {
	if db == nil {
		return fmt.Errorf("migrate: db is nil")
	}
	if err := ensureTable(ctx, db); err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	for _, m := range migrations {
		if m.Version <= 0 {
			return fmt.Errorf("migrate: invalid version for %q", m.Name)
		}
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

func ensureTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INT NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  applied_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	if err != nil {
		return fmt.Errorf("migrate ensure table: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate list: %w", err)
	}
	defer rows.Close()
	out := make(map[int]struct{})
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate scan: %w", err)
		}
		out[v] = struct{}{}
	}
	return out, rows.Err()
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate begin %d: %w", m.Version, err)
	}
	defer func() { _ = tx.Rollback() }()

	// go-mysql 默认禁止单次 Exec 多语句；按 ; 拆开逐条执行（无需开 multiStatements）。
	stmts := splitSQL(m.Up)
	if len(stmts) == 0 {
		return fmt.Errorf("migrate up %d (%s): empty SQL", m.Version, m.Name)
	}
	for i, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate up %d (%s) stmt %d: %w", m.Version, m.Name, i+1, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.Version, m.Name, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("migrate record %d: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate commit %d: %w", m.Version, err)
	}
	return nil
}

// splitSQL splits a migration batch into individual statements.
// DDL in this repo does not embed ; inside string literals.
func splitSQL(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
