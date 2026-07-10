package equity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

const yahooChartBase = "https://query1.finance.yahoo.com/v8/finance/chart/"

var yahooKlineSymbols = map[string]string{
	"ks11": "^KS11",
	"n225": "^N225",
}

// FetchYahooKlines loads daily candles for global indices without A/H/US Baidu kline support.
func FetchYahooKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	ctx, cancel := context.WithTimeout(context.Background(), indexKlineFetchBudget)
	defer cancel()
	return fetchYahooKlinesCtx(ctx, client, def, interval, limit)
}

func fetchYahooKlinesCtx(ctx context.Context, client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	symbol, ok := yahooKlineSymbol(def.ID)
	if !ok {
		return nil, fmt.Errorf("%s yahoo kline: symbol not mapped", def.ID)
	}
	interval = strings.ToLower(strings.TrimSpace(interval))
	if interval == "" {
		interval = "1d"
	}
	if interval != "1d" {
		return nil, fmt.Errorf("%s yahoo kline: interval %s not supported", def.ID, interval)
	}
	if client == nil {
		client = eastmoneyKlineHTTPClient
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1000 {
		limit = 1000
	}

	q := url.Values{}
	q.Set("interval", "1d")
	q.Set("range", yahooRangeForLimit(limit))
	reqURL := yahooChartBase + url.PathEscape(symbol) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s yahoo kline request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s yahoo kline http %d", def.ID, resp.StatusCode)
	}
	candles, err := parseYahooChart(body)
	if err != nil {
		return nil, fmt.Errorf("%s yahoo kline: %w", def.ID, err)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("%s yahoo kline: no candles", def.ID)
	}
	return candles, nil
}

func yahooKlineSymbol(id string) (string, bool) {
	symbol, ok := yahooKlineSymbols[strings.ToLower(strings.TrimSpace(id))]
	return symbol, ok
}

func yahooRangeForLimit(limit int) string {
	switch {
	case limit <= 30:
		return "3mo"
	case limit <= 90:
		return "6mo"
	case limit <= 180:
		return "1y"
	case limit <= 400:
		return "2y"
	default:
		return "5y"
	}
}

func parseYahooChart(body []byte) ([]binance.Candle, error) {
	var parsed struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []float64 `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty chart result")
	}
	block := parsed.Chart.Result[0]
	if len(block.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("empty quote block")
	}
	quote := block.Indicators.Quote[0]
	out := make([]binance.Candle, 0, len(block.Timestamp))
	for i, ts := range block.Timestamp {
		open := valueAt(quote.Open, i)
		high := valueAt(quote.High, i)
		low := valueAt(quote.Low, i)
		closep := valueAt(quote.Close, i)
		if closep <= 0 && open <= 0 {
			continue
		}
		out = append(out, binance.Candle{
			Time:   ts,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closep,
			Volume: valueAt(quote.Volume, i),
			Closed: true,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no candles")
	}
	return out, nil
}

func valueAt(values []float64, idx int) float64 {
	if idx < 0 || idx >= len(values) {
		return 0
	}
	return values[idx]
}
