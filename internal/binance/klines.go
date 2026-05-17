package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var restBase = "https://api.binance.com"

// Candle is one OHLCV bar for charting.
type Candle struct {
	Time   int64   `json:"time"` // unix seconds (UTC)
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

var allowedIntervals = map[string]struct{}{
	"1m": {}, "5m": {}, "15m": {}, "1h": {}, "4h": {}, "1d": {}, "1w": {},
}

// NormalizeInterval validates interval string.
func NormalizeInterval(interval string) (string, error) {
	interval = strings.ToLower(strings.TrimSpace(interval))
	if _, ok := allowedIntervals[interval]; !ok {
		return "", fmt.Errorf("unsupported interval: %s", interval)
	}
	return interval, nil
}

// SymbolUSDT returns binance pair e.g. BTC -> BTCUSDT.
func SymbolUSDT(base string) string {
	return strings.ToUpper(strings.TrimSpace(base)) + "USDT"
}

// FetchKlines loads historical candles from Binance spot REST.
func FetchKlines(baseSymbol, interval string, limit int) ([]Candle, error) {
	return fetchKlines(baseSymbol, interval, limit, 0)
}

// FetchKlineOpenAt returns the open of the 1m candle whose open time is start (inclusive).
// Used for Beijing-midnight day open: start = DayStartShanghai(now).
func FetchKlineOpenAt(baseSymbol string, start time.Time) (float64, error) {
	candles, err := fetchKlines(baseSymbol, "1m", 1, start.UnixMilli())
	if err != nil {
		return 0, err
	}
	if len(candles) == 0 || candles[0].Open <= 0 {
		return 0, fmt.Errorf("binance kline open at %s: no candle", start.Format(time.RFC3339))
	}
	return candles[0].Open, nil
}

func fetchKlines(baseSymbol, interval string, limit int, startTimeMs int64) ([]Candle, error) {
	interval, err := NormalizeInterval(interval)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	pair := SymbolUSDT(baseSymbol)
	q := url.Values{}
	q.Set("symbol", pair)
	q.Set("interval", interval)
	q.Set("limit", strconv.Itoa(limit))
	if startTimeMs > 0 {
		q.Set("startTime", strconv.FormatInt(startTimeMs, 10))
	}

	reqURL := restBase + "/api/v3/klines?" + q.Encode()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("binance klines request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance klines http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var raw [][]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance klines parse: %w", err)
	}

	out := make([]Candle, 0, len(raw))
	for _, row := range raw {
		if len(row) < 6 {
			continue
		}
		var openTimeMs int64
		if err := json.Unmarshal(row[0], &openTimeMs); err != nil {
			continue
		}
		open, _ := parseFloatRaw(row[1])
		high, _ := parseFloatRaw(row[2])
		low, _ := parseFloatRaw(row[3])
		closep, _ := parseFloatRaw(row[4])
		vol, _ := parseFloatRaw(row[5])
		out = append(out, Candle{
			Time:   openTimeMs / 1000,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closep,
			Volume: vol,
		})
	}
	return out, nil
}

func parseFloatRaw(r json.RawMessage) (float64, error) {
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return strconv.ParseFloat(s, 64)
	}
	var f float64
	if err := json.Unmarshal(r, &f); err != nil {
		return 0, err
	}
	return f, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
