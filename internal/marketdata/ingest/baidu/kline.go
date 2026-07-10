package baidu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

const stockQuotationPath = "/selfselect/getstockquotation"

// FetchKlines loads index candles from Baidu Finance HTTP API.
func FetchKlines(client *http.Client, cfg Config, ref IndexRef, interval string, limit int) ([]binance.Candle, error) {
	cfg = normalizeConfig(cfg)
	if !cfg.Enabled {
		return nil, fmt.Errorf("baidu: disabled")
	}
	if !ref.valid() {
		return nil, fmt.Errorf("%s: no baidu mapping", ref.ID)
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1000 {
		limit = 1000
	}
	query, err := klineParams(ref, interval)
	if err != nil {
		return nil, err
	}
	envelope, err := getJSON(client, cfg.BaseURL, stockQuotationPath, query)
	if err != nil {
		return nil, err
	}
	marketData, keys, err := decodeStockQuotationResult(envelope.Result)
	if err != nil {
		return nil, fmt.Errorf("%s baidu kline parse: %w", ref.ID, err)
	}
	candles, err := parseMarketData(marketData, keys)
	if err != nil {
		return nil, fmt.Errorf("%s baidu kline: %w", ref.ID, err)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("%s baidu kline: no candles", ref.ID)
	}
	if err := validateKlinePrices(ref, candles); err != nil {
		return nil, err
	}
	return candles, nil
}

func validateKlinePrices(ref IndexRef, candles []binance.Candle) error {
	if ref.MinPrice <= 0 && ref.MaxPrice <= 0 {
		return nil
	}
	last := candles[len(candles)-1]
	price := last.Close
	if price <= 0 {
		price = last.Open
	}
	if price <= 0 {
		return nil
	}
	if err := ref.validatePrice(price); err != nil {
		return fmt.Errorf("%s baidu kline: %w", ref.ID, err)
	}
	return nil
}

// FetchKlinesCtx is a context-aware wrapper for index kline fetch.
func FetchKlinesCtx(ctx context.Context, client *http.Client, cfg Config, ref IndexRef, interval string, limit int) ([]binance.Candle, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return FetchKlines(client, cfg, ref, interval, limit)
}

func decodeStockQuotationResult(raw json.RawMessage) (marketData, keys string, err error) {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", "", fmt.Errorf("empty result")
	}
	if raw[0] == '[' {
		return "", "", fmt.Errorf("empty result")
	}
	var result stockQuotationResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", err
	}
	if result.NewMarketData != nil {
		marketData = strings.TrimSpace(result.NewMarketData.MarketData)
		keys = decodeMarketDataKeys(result.NewMarketData.Keys)
	}
	if marketData == "" {
		marketData = strings.TrimSpace(result.MarketData)
	}
	if keys == "" {
		keys = strings.TrimSpace(result.Keys)
	}
	if marketData == "" {
		return "", "", fmt.Errorf("empty marketData")
	}
	return marketData, keys, nil
}

func decodeMarketDataKeys(raw json.RawMessage) string {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	if raw[0] == '"' {
		var keys string
		if err := json.Unmarshal(raw, &keys); err == nil {
			return strings.TrimSpace(keys)
		}
	}
	var keyList []string
	if err := json.Unmarshal(raw, &keyList); err == nil {
		return strings.Join(keyList, ",")
	}
	return ""
}

func bytesTrim(raw json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(raw)))
}

func parseMarketData(marketData, keys string) ([]binance.Candle, error) {
	marketData = strings.TrimSpace(marketData)
	if marketData == "" {
		return nil, fmt.Errorf("empty marketData")
	}
	keyList := splitCSV(keys)
	rows := strings.Split(marketData, ";")
	out := make([]binance.Candle, 0, len(rows))
	for _, row := range rows {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		candle, err := parseMarketDataRow(row, keyList)
		if err != nil {
			continue
		}
		out = append(out, candle)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no parsed candles")
	}
	return out, nil
}

func parseMarketDataRow(row string, keys []string) (binance.Candle, error) {
	parts := splitCSV(row)
	values := make(map[string]string, len(keys))
	for i, key := range keys {
		if key == "" || i >= len(parts) {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(parts[i])
	}
	get := func(names ...string) string {
		for _, name := range names {
			if v, ok := values[strings.ToLower(name)]; ok {
				return v
			}
		}
		return ""
	}
	tsRaw := get("timestamp", "time")
	if tsRaw == "" {
		return binance.Candle{}, fmt.Errorf("missing timestamp")
	}
	ts, err := parseTimestamp(tsRaw)
	if err != nil {
		return binance.Candle{}, err
	}
	open, _ := strconv.ParseFloat(strings.ReplaceAll(get("open"), ",", ""), 64)
	high, _ := strconv.ParseFloat(strings.ReplaceAll(get("high"), ",", ""), 64)
	low, _ := strconv.ParseFloat(strings.ReplaceAll(get("low"), ",", ""), 64)
	closep, _ := strconv.ParseFloat(strings.ReplaceAll(get("close"), ",", ""), 64)
	volume, _ := strconv.ParseFloat(strings.ReplaceAll(get("volume"), ",", ""), 64)
	if closep <= 0 && open <= 0 {
		return binance.Candle{}, fmt.Errorf("empty ohlc")
	}
	return binance.Candle{
		Time:   ts,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closep,
		Volume: volume,
		Closed: true,
	}, nil
}

func parseTimestamp(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty timestamp")
	}
	if ts, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if ts > 1_000_000_000_000 {
			return ts / 1000, nil
		}
		return ts, nil
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("unsupported timestamp: %s", raw)
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
