// Package mysql opens a shared *sql.DB for business modules (users, alerts, portfolio).
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lzqqdy/marketpulse/internal/config"
)

// Open creates a pooled MySQL connection and verifies it with Ping.
func Open(cfg config.MySQLConfig) (*sql.DB, error) {
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
