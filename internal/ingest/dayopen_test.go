package ingest

import (
	"testing"
	"time"

	"github.com/lzqqdy/marketpulse/internal/binance"
)

func Test_dayOpenCache_changePct(t *testing.T) {
	c := newDayOpenCache()
	date := binance.ExchangeDayKeyUTC(time.Now())
	c.replace(date, map[string]float64{"BTC": 100})

	pct, ok := c.changePct("BTC", 101, time.Now())
	if !ok || pct < 0.99 || pct > 1.01 {
		t.Fatalf("pct=%v ok=%v", pct, ok)
	}

	c.replace("2020-01-01", map[string]float64{"BTC": 100})
	_, ok = c.changePct("BTC", 101, time.Now())
	if ok {
		t.Fatal("stale date should miss")
	}
}
