package mysql

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/config"
)

func TestOpen_disabledDSNShape(t *testing.T) {
	cfg := config.MySQLConfig{
		Host:            "db.example",
		Port:            3307,
		User:            "mp",
		Password:        "secret",
		Database:        "marketpulse",
		Params:          "parseTime=true&loc=Local&charset=utf8mb4",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 5 * time.Minute,
	}
	got := cfg.DSN()
	want := "mp:secret@tcp(db.example:3307)/marketpulse?parseTime=true&loc=Local&charset=utf8mb4"
	if got != want {
		t.Fatalf("dsn: got %q want %q", got, want)
	}
}
