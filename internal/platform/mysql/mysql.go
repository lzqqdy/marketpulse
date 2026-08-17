package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lzqqdy/marketpulse/internal/config"
)

var dbNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Open creates a pooled MySQL connection and verifies it with Ping.
func Open(cfg config.MySQLConfig) (*sql.DB, error) {
	if err := ensureDatabase(cfg); err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := pingWithRetry(db, 8, 2*time.Second); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return db, nil
}

func ensureDatabase(cfg config.MySQLConfig) error {
	if cfg.Database == "" {
		return nil
	}
	if !dbNameRe.MatchString(cfg.Database) {
		return fmt.Errorf("mysql: invalid database name %q", cfg.Database)
	}
	admin := cfg
	admin.Database = ""
	db, err := sql.Open("mysql", admin.DSN())
	if err != nil {
		return fmt.Errorf("mysql admin open: %w", err)
	}
	defer db.Close()
	if err := pingWithRetry(db, 8, 2*time.Second); err != nil {
		return fmt.Errorf("mysql admin ping: %w", err)
	}
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.Database)
	if _, err := db.Exec(q); err != nil {
		return fmt.Errorf("mysql create database: %w", err)
	}
	return nil
}

// pingWithRetry wakes WeChat Cloud Run MySQL after auto-pause.
func pingWithRetry(db *sql.DB, attempts int, delay time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err == nil {
			return nil
		}
		if i < attempts {
			time.Sleep(delay)
		}
	}
	return err
}
