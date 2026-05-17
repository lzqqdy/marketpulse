package binance

import (
	"testing"
	"time"
)

func TestExchangeDayStartUTC(t *testing.T) {
	// 2026-05-17 00:02 CST is still Binance exchange day 2026-05-16.
	cst := time.Date(2026, 5, 17, 0, 2, 0, 0, Shanghai)
	start := ExchangeDayStartUTC(cst)
	if !start.Equal(time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start: %v", start)
	}
	if ExchangeDayKeyUTC(cst) != "2026-05-16" {
		t.Fatalf("key: %s", ExchangeDayKeyUTC(cst))
	}

	// Binance flips to a new daily candle at 08:00 CST.
	afterOpen := time.Date(2026, 5, 17, 8, 1, 0, 0, Shanghai)
	if ExchangeDayKeyUTC(afterOpen) != "2026-05-17" {
		t.Fatalf("key after open: %s", ExchangeDayKeyUTC(afterOpen))
	}
}

func TestNextExchangeDayStartUTC(t *testing.T) {
	cst := time.Date(2026, 5, 17, 0, 2, 0, 0, Shanghai)
	next := NextExchangeDayStartUTC(cst)
	if !next.In(Shanghai).Equal(time.Date(2026, 5, 17, 8, 0, 0, 0, Shanghai)) {
		t.Fatalf("next start: %v", next.In(Shanghai))
	}
}
