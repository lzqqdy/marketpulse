package binance

import "time"

// Shanghai is used for UI-facing local time calculations and tests.
var Shanghai *time.Location

func init() {
	var err error
	Shanghai, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		Shanghai = time.FixedZone("CST", 8*3600)
	}
}

// ExchangeDayStartUTC returns the Binance spot daily candle boundary.
// Binance daily candles open at 00:00 UTC, which is 08:00 in Asia/Shanghai.
func ExchangeDayStartUTC(now time.Time) time.Time {
	t := now.UTC()
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ExchangeDayKeyUTC is YYYY-MM-DD for the Binance exchange day of now.
func ExchangeDayKeyUTC(now time.Time) string {
	return ExchangeDayStartUTC(now).Format("2006-01-02")
}

// NextExchangeDayStartUTC is the next Binance daily candle boundary.
func NextExchangeDayStartUTC(now time.Time) time.Time {
	return ExchangeDayStartUTC(now).Add(24 * time.Hour)
}
