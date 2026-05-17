package equity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/binance"
	"github.com/lzqqdy/marketpulse/internal/store"
)

// IndexDef maps internal id to Yahoo symbol and display name.
type IndexDef struct {
	ID          string
	Name        string
	Symbol      string
	StooqSymbol string
}

// DefaultIndices is the global index watchlist plus international gold.
var DefaultIndices = []IndexDef{
	{ID: "sh000001", Name: "上证", Symbol: "000001.SS"},
	{ID: "sz399001", Name: "深证", Symbol: "399001.SZ"},
	{ID: "hsi", Name: "恒生", Symbol: "^HSI", StooqSymbol: "^hsi"},
	{ID: "dji", Name: "道琼斯", Symbol: "^DJI", StooqSymbol: "^dji"},
	{ID: "ixic", Name: "纳斯达克", Symbol: "^IXIC", StooqSymbol: "^ndq"},
	{ID: "gspc", Name: "标普500", Symbol: "^GSPC", StooqSymbol: "^spx"},
	{ID: "n225", Name: "日经225", Symbol: "^N225", StooqSymbol: "^nkx"},
	{ID: "ftse", Name: "富时100", Symbol: "^FTSE", StooqSymbol: "^ukx"},
	{ID: "gdaxi", Name: "DAX", Symbol: "^GDAXI", StooqSymbol: "^dax"},
	{ID: "fchi", Name: "CAC40", Symbol: "^FCHI", StooqSymbol: "^cac"},
	{ID: "ks11", Name: "KOSPI", Symbol: "^KS11"},
	{ID: "twii", Name: "台湾加权", Symbol: "^TWII"},
	{ID: "nsei", Name: "Nifty 50", Symbol: "^NSEI"},
	{ID: "axjo", Name: "ASX 200", Symbol: "^AXJO"},
	{ID: "gold", Name: "国际黄金", Symbol: "GC=F", StooqSymbol: "gc.f"},
}

const chartBase = "https://query2.finance.yahoo.com/v8/finance/chart/"

const (
	yahooRequestGap    = 600 * time.Millisecond
	yahooMaxRetries    = 2
	yahooRetryBackoff  = 3 * time.Second
	yahooAbortAfter429 = 3 // 连续 429 则中止本轮剩余标的
)

// DefaultIndexByID finds a configured index definition by internal id.
func DefaultIndexByID(id string) (IndexDef, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, def := range DefaultIndices {
		if strings.ToLower(def.ID) == id {
			return def, true
		}
	}
	return IndexDef{}, false
}

// ResolveDefs maps configured index ids to Yahoo definitions.
func ResolveDefs(ids []string) []IndexDef {
	if len(ids) == 0 {
		return DefaultIndices
	}
	out := make([]IndexDef, 0, len(ids))
	for _, id := range ids {
		if def, ok := DefaultIndexByID(id); ok {
			out = append(out, def)
		}
	}
	return out
}

// FetchAll loads index quotes via Yahoo chart API (2d range for change%).
func FetchAll(client *http.Client, defs []IndexDef) ([]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(defs) == 0 {
		defs = DefaultIndices
	}
	now := time.Now().UTC()
	out := make([]store.IndexQuote, 0, len(defs))
	var firstErr error
	consecutive429 := 0

	for i, def := range defs {
		if i > 0 {
			time.Sleep(yahooRequestGap)
		}
		if consecutive429 >= yahooAbortAfter429 {
			break
		}
		q, err := fetchOne(client, def, now)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if IsRateLimitErr(err) {
				consecutive429++
			} else {
				consecutive429 = 0
			}
			continue
		}
		consecutive429 = 0
		out = append(out, q)
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, firstErr
}

func IsRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(msg, "503")
}

func fetchOne(client *http.Client, def IndexDef, now time.Time) (store.IndexQuote, error) {
	u := chartBase + url.PathEscape(def.Symbol) + "?interval=1d&range=2d"
	var lastErr error
	for attempt := 0; attempt < yahooMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(yahooRetryBackoff)
		}
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return store.IndexQuote{}, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s request: %w", def.ID, err)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			lastErr = fmt.Errorf("%s http %d", def.ID, resp.StatusCode)
			return store.IndexQuote{}, lastErr
		}
		if resp.StatusCode != http.StatusOK {
			return store.IndexQuote{}, fmt.Errorf("%s http %d", def.ID, resp.StatusCode)
		}

		var parsed yahooChart
		if err := json.Unmarshal(body, &parsed); err != nil {
			return store.IndexQuote{}, fmt.Errorf("%s parse: %w", def.ID, err)
		}
		price, chg, err := parsed.latestChange()
		if err != nil {
			return store.IndexQuote{}, fmt.Errorf("%s: %w", def.ID, err)
		}

		return store.IndexQuote{
			ID:        def.ID,
			Name:      def.Name,
			Price:     price,
			ChangePct: chg,
			UpdatedAt: now,
		}, nil
	}
	if lastErr != nil {
		return store.IndexQuote{}, lastErr
	}
	return store.IndexQuote{}, fmt.Errorf("%s: exhausted retries", def.ID)
}

// FetchKlines loads historical candles for an index or gold symbol via Yahoo chart API.
func FetchKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	if client == nil {
		client = http.DefaultClient
	}
	queryInterval, queryRange, err := normalizeKlineInterval(interval)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 300
	}
	if limit > 1000 {
		limit = 1000
	}

	q := url.Values{}
	q.Set("interval", queryInterval)
	q.Set("range", queryRange)
	q.Set("events", "history")
	reqURL := chartBase + url.PathEscape(def.Symbol) + "?" + q.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s kline request: %w", def.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s kline http %d", def.ID, resp.StatusCode)
	}

	var parsed yahooKlineChart
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%s kline parse: %w", def.ID, err)
	}
	candles, err := parsed.candles()
	if err != nil {
		return nil, fmt.Errorf("%s kline: %w", def.ID, err)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	return candles, nil
}

func normalizeKlineInterval(interval string) (queryInterval, queryRange string, err error) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "15m":
		return "15m", "5d", nil
	case "1h":
		return "1h", "3mo", nil
	case "1d", "":
		return "1d", "1y", nil
	case "1w":
		return "1wk", "5y", nil
	default:
		return "", "", fmt.Errorf("unsupported index interval: %s", interval)
	}
}

type yahooChart struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

type yahooKlineChart struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

func (y *yahooKlineChart) candles() ([]binance.Candle, error) {
	if len(y.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	r := y.Chart.Result[0]
	if len(r.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("empty quote")
	}
	q := r.Indicators.Quote[0]
	out := make([]binance.Candle, 0, len(r.Timestamp))
	for i, ts := range r.Timestamp {
		open, ok := ptrAt(q.Open, i)
		if !ok {
			continue
		}
		high, ok := ptrAt(q.High, i)
		if !ok {
			continue
		}
		low, ok := ptrAt(q.Low, i)
		if !ok {
			continue
		}
		closep, ok := ptrAt(q.Close, i)
		if !ok {
			continue
		}
		volume, _ := ptrAt(q.Volume, i)
		out = append(out, binance.Candle{
			Time:   ts,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closep,
			Volume: volume,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no candles")
	}
	return out, nil
}

func ptrAt(vals []*float64, i int) (float64, bool) {
	if i < 0 || i >= len(vals) || vals[i] == nil {
		return 0, false
	}
	return *vals[i], true
}

func (y *yahooChart) latestChange() (price, changePct float64, err error) {
	if len(y.Chart.Result) == 0 {
		return 0, 0, fmt.Errorf("empty result")
	}
	r := y.Chart.Result[0]
	price = r.Meta.RegularMarketPrice
	if price <= 0 {
		closes := r.Indicators.Quote[0].Close
		for i := len(closes) - 1; i >= 0; i-- {
			if closes[i] > 0 {
				price = closes[i]
				break
			}
		}
	}
	if price <= 0 {
		return 0, 0, fmt.Errorf("no price")
	}

	prev := r.Meta.PreviousClose
	if prev <= 0 {
		prev = r.Meta.ChartPreviousClose
	}
	closes := r.Indicators.Quote[0].Close
	valid := compact(closes)
	if len(valid) >= 2 {
		prev = valid[len(valid)-2]
	} else if len(valid) == 1 && prev <= 0 {
		prev = valid[0]
	}
	if prev <= 0 {
		return price, 0, nil
	}
	changePct = (price - prev) / prev * 100
	return price, changePct, nil
}

func compact(vals []float64) []float64 {
	out := make([]float64, 0, len(vals))
	for _, v := range vals {
		if v > 0 {
			out = append(out, v)
		}
	}
	return out
}
