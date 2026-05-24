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

// ShanghaiDayStartUTC returns the UTC instant for 00:00 in Asia/Shanghai.
func ShanghaiDayStartUTC(now time.Time) time.Time {
	t := now.In(Shanghai)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Shanghai).UTC()
}

// ShanghaiDayKey is YYYY-MM-DD for the natural day in Asia/Shanghai.
func ShanghaiDayKey(now time.Time) string {
	return now.In(Shanghai).Format("2006-01-02")
}

// NextShanghaiDayStartUTC is the next 00:00 boundary in Asia/Shanghai.
func NextShanghaiDayStartUTC(now time.Time) time.Time {
	return ShanghaiDayStartUTC(now).Add(24 * time.Hour)
}
