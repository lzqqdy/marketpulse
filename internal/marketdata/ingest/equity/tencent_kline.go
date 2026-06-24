package equity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

const tencentKlineBase = "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get"

var tencentKlineHTTPClient = &http.Client{Timeout: eastmoneyKlineTimeout}

// FetchTencentKlines loads daily index/commodity candles from Tencent fqkline.
func FetchTencentKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	ctx, cancel := context.WithTimeout(context.Background(), indexKlineFetchBudget)
	defer cancel()
	return fetchTencentKlinesCtx(ctx, client, def, interval, limit)
}

func fetchTencentKlinesCtx(ctx context.Context, client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	if client == nil {
		client = tencentKlineHTTPClient
	}
	interval = strings.ToLower(strings.TrimSpace(interval))
	if interval == "" {
		interval = "1d"
	}
	if interval != "1d" {
		return nil, fmt.Errorf("%s tencent kline: interval %s not supported", def.ID, interval)
	}
	symbol, ok := tencentKlineSymbol(def)
	if !ok {
		return nil, fmt.Errorf("%s tencent kline: symbol not mapped", def.ID)
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1000 {
		limit = 1000
	}

	param := fmt.Sprintf("%s,day,,,%d,qfq", symbol, limit)
	reqURL := tencentKlineBase + "?" + url.Values{"param": {param}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://gu.qq.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s tencent kline request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s tencent kline http %d", def.ID, resp.StatusCode)
	}

	var parsed tencentKlineResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%s tencent kline parse: %w", def.ID, err)
	}
	if parsed.Code != 0 || parsed.Msg == "param error" {
		return nil, fmt.Errorf("%s tencent kline: %s", def.ID, parsed.Msg)
	}
	rows := parsed.dayRows(symbol)
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s tencent kline: no candles", def.ID)
	}
	candles, err := parseTencentDayRows(rows)
	if err != nil {
		return nil, fmt.Errorf("%s tencent kline: %w", def.ID, err)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	return candles, nil
}

type tencentKlineResponse struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data map[string]any `json:"data"`
}

func (r tencentKlineResponse) dayRows(symbol string) [][]any {
	if r.Data == nil {
		return nil
	}
	raw, ok := r.Data[symbol]
	if !ok {
		return nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	day, ok := block["day"]
	if !ok {
		return nil
	}
	rows, ok := day.([]any)
	if !ok {
		return nil
	}
	out := make([][]any, 0, len(rows))
	for _, row := range rows {
		cols, ok := row.([]any)
		if !ok || len(cols) < 5 {
			continue
		}
		out = append(out, cols)
	}
	return out
}

func parseTencentDayRows(rows [][]any) ([]binance.Candle, error) {
	out := make([]binance.Candle, 0, len(rows))
	for _, cols := range rows {
		ts, err := parseTencentKlineTime(fmt.Sprint(cols[0]))
		if err != nil {
			continue
		}
		open, err := strconv.ParseFloat(fmt.Sprint(cols[1]), 64)
		if err != nil {
			continue
		}
		closep, err := strconv.ParseFloat(fmt.Sprint(cols[2]), 64)
		if err != nil {
			continue
		}
		high, err := strconv.ParseFloat(fmt.Sprint(cols[3]), 64)
		if err != nil {
			continue
		}
		low, err := strconv.ParseFloat(fmt.Sprint(cols[4]), 64)
		if err != nil {
			continue
		}
		volume, _ := strconv.ParseFloat(fmt.Sprint(cols[5]), 64)
		out = append(out, binance.Candle{
			Time:   ts,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closep,
			Volume: volume,
			Closed: true,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no candles")
	}
	return out, nil
}

func parseTencentKlineTime(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.ParseInLocation("2006-01-02", raw, tencentQuoteLocation); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("unsupported time: %s", raw)
}

func tencentKlineSymbol(def IndexDef) (string, bool) {
	switch def.ID {
	case "crude":
		return "usCL", true
	case "silver":
		return "usSI", true
	case "n225", "ks11", "gold":
		return "", false
	}
	sym := strings.TrimSpace(def.TencentSymbol)
	if sym == "" {
		return "", false
	}
	if strings.HasPrefix(sym, "s_") {
		return sym[2:], true
	}
	if strings.HasPrefix(sym, "hf_") || strings.HasPrefix(sym, "gz") {
		return "", false
	}
	return sym, true
}
