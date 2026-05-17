package binance

import "time"

// Shanghai is the market day boundary for "today" change (UTC+8).
var Shanghai *time.Location

func init() {
	var err error
	Shanghai, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		Shanghai = time.FixedZone("CST", 8*3600)
	}
}

// DayStartShanghai returns today 00:00:00 in Asia/Shanghai for the given instant.
func DayStartShanghai(now time.Time) time.Time {
	t := now.In(Shanghai)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Shanghai)
}

// DayKeyShanghai is YYYY-MM-DD for the Shanghai calendar day of now.
func DayKeyShanghai(now time.Time) string {
	return DayStartShanghai(now).Format("2006-01-02")
}

// NextDayStartShanghai is the next midnight after now in Shanghai.
func NextDayStartShanghai(now time.Time) time.Time {
	return DayStartShanghai(now).Add(24 * time.Hour)
}
