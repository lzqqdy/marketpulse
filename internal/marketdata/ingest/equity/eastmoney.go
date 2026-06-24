package equity

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const (
	eastmoneyQuoteBase = "https://push2.eastmoney.com/api/qt/stock/get"
	eastmoneyKlineBase = "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	eastmoneyUT        = "fa5fd1943c7b386f172d6893dbfba10b"
	eastmoneyGap       = 250 * time.Millisecond
	eastmoneyAttempts  = 3
)

// eastmoneyKlineHTTPClient is used for kline history; responses must stay bounded
// (beg=0 returns full history and can exceed 700KB, causing EOF on slow links).
var eastmoneyKlineHTTPClient = &http.Client{Timeout: 30 * time.Second}

type eastmoneyQuoteResponse struct {
	RC   int `json:"rc"`
	Data struct {
		Code      string  `json:"f57"`
		Name      string  `json:"f58"`
		Price     float64 `json:"f43"`
		High      float64 `json:"f44"`
		Low       float64 `json:"f45"`
		Open      float64 `json:"f46"`
		Volume    float64 `json:"f47"`
		PrevClose float64 `json:"f60"`
		UpdatedAt int64   `json:"f86"`
		Change    float64 `json:"f169"`
		ChangePct float64 `json:"f170"`
		Amplitude float64 `json:"f171"`
	} `json:"data"`
}

// FetchEastmoneyQuotes loads index quotes one-by-one. This is intentionally
// slower than batching because the single-symbol endpoint proved steadier.
func FetchEastmoneyQuotes(client *http.Client, defs []IndexDef) (map[string]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	now := time.Now().UTC()
	out := make(map[string]store.IndexQuote, len(defs))
	var firstErr error
	for i, def := range defs {
		if strings.TrimSpace(def.EastmoneySecID) == "" {
			continue
		}
		if i > 0 {
			time.Sleep(eastmoneyGap)
		}
		row, err := fetchEastmoneyOne(client, def, now)
		if err != nil {
			slog.Warn("equity symbol fetch failed", "provider", "eastmoney", "id", def.ID, "secid", def.EastmoneySecID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		out[row.ID] = row
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, firstErr
}

func fetchEastmoneyOne(client *http.Client, def IndexDef, now time.Time) (store.IndexQuote, error) {
	q := url.Values{}
	q.Set("fltt", "2")
	q.Set("secid", def.EastmoneySecID)
	q.Set("fields", "f57,f58,f43,f44,f45,f46,f47,f48,f60,f169,f170,f171,f86,f152")
	req, err := http.NewRequest(http.MethodGet, eastmoneyQuoteBase+"?"+q.Encode(), nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	setEastmoneyHeaders(req, def)

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "eastmoney", "id", def.ID, "secid", def.EastmoneySecID, "err", err)
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "eastmoney", "id", def.ID, "secid", def.EastmoneySecID, "status", resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney http %d", def.ID, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney http %d", def.ID, resp.StatusCode)
	}

	var parsed eastmoneyQuoteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney parse: %w", def.ID, err)
	}
	if parsed.RC != 0 {
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney rc %d", def.ID, parsed.RC)
	}
	if err := validatePrice(def, parsed.Data.Price); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s eastmoney: %w", def.ID, err)
	}
	updatedAt := now
	if parsed.Data.UpdatedAt > 0 {
		updatedAt = time.Unix(parsed.Data.UpdatedAt, 0).UTC()
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     parsed.Data.Price,
		ChangePct: parsed.Data.ChangePct,
		Source:    "eastmoney",
		FetchedAt: now,
		UpdatedAt: updatedAt,
	}, nil
}

// FetchEastmoneyKlines loads historical candles for an index or gold symbol.
func FetchEastmoneyKlines(client *http.Client, def IndexDef, interval string, limit int) ([]binance.Candle, error) {
	if client == nil {
		client = eastmoneyKlineHTTPClient
	}
	klt, err := normalizeEastmoneyKlineInterval(interval)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(def.EastmoneySecID) == "" {
		return nil, fmt.Errorf("%s: eastmoney secid not configured", def.ID)
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1000 {
		limit = 1000
	}

	var lastErr error
	for _, fetchLimit := range klineLimitAttempts(limit) {
		for attempt := 1; attempt <= eastmoneyAttempts; attempt++ {
			candles, err := fetchEastmoneyKlinesOnce(client, def, klt, fetchLimit)
			if err == nil {
				return candles, nil
			}
			lastErr = err
			if attempt < eastmoneyAttempts {
				time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
			}
		}
	}
	return nil, lastErr
}

func fetchEastmoneyKlinesOnce(client *http.Client, def IndexDef, klt string, limit int) ([]binance.Candle, error) {
	now := time.Now().UTC()
	beg, end := eastmoneyKlineBegEnd(klt, limit, now)
	q := url.Values{}
	q.Set("secid", def.EastmoneySecID)
	q.Set("klt", klt)
	q.Set("fqt", "0")
	q.Set("lmt", strconv.Itoa(limit))
	q.Set("beg", beg)
	q.Set("end", end)
	q.Set("ut", eastmoneyUT)
	q.Set("rtntype", "6")
	q.Set("fields1", "f1,f2,f3,f4,f5,f6")
	q.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61")
	req, err := http.NewRequest(http.MethodGet, eastmoneyKlineBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	setEastmoneyHeaders(req, def)
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s eastmoney kline request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s eastmoney kline http %d", def.ID, resp.StatusCode)
	}
	var parsed eastmoneyKlineResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("%s eastmoney kline parse: %w", def.ID, err)
	}
	if parsed.RC != 0 || parsed.Data == nil {
		return nil, fmt.Errorf("%s eastmoney kline rc %d", def.ID, parsed.RC)
	}
	candles, err := parsed.Data.candles()
	if err != nil {
		return nil, fmt.Errorf("%s eastmoney kline: %w", def.ID, err)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	return candles, nil
}

func klineLimitAttempts(limit int) []int {
	candidates := []int{limit, 120, 60, 30}
	out := make([]int, 0, len(candidates))
	seen := map[int]struct{}{}
	for _, n := range candidates {
		if n <= 0 || n > limit {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

type eastmoneyKlineResponse struct {
	RC   int                 `json:"rc"`
	Data *eastmoneyKlineData `json:"data"`
}

type eastmoneyKlineData struct {
	Klines []string `json:"klines"`
}

func (d *eastmoneyKlineData) candles() ([]binance.Candle, error) {
	out := make([]binance.Candle, 0, len(d.Klines))
	for _, raw := range d.Klines {
		parts := strings.Split(raw, ",")
		if len(parts) < 6 {
			continue
		}
		ts, err := parseEastmoneyKlineTime(parts[0])
		if err != nil {
			continue
		}
		open, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}
		closep, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			continue
		}
		high, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			continue
		}
		low, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			continue
		}
		volume, _ := strconv.ParseFloat(parts[5], 64)
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

// eastmoneyKlineBegEnd returns a bounded date window. Eastmoney ignores lmt when
// beg=0/end=20500101 and returns the entire history (9000+ daily bars).
func eastmoneyKlineBegEnd(klt string, limit int, now time.Time) (beg, end string) {
	if limit <= 0 {
		limit = 30
	}
	end = now.Format("20060102")
	lookbackDays := eastmoneyKlineLookbackDays(klt, limit)
	beg = now.AddDate(0, 0, -lookbackDays).Format("20060102")
	return beg, end
}

func eastmoneyKlineLookbackDays(klt string, limit int) int {
	switch klt {
	case "15":
		days := limit/20 + 7
		if days < 14 {
			return 14
		}
		return days
	case "60":
		days := limit/6 + 10
		if days < 21 {
			return 21
		}
		return days
	case "102":
		days := limit*7 + 21
		if days < 60 {
			return 60
		}
		return days
	default:
		days := limit * 2
		if days < 30 {
			return 30
		}
		return days
	}
}

func normalizeEastmoneyKlineInterval(interval string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "15m":
		return "15", nil
	case "1h":
		return "60", nil
	case "1d", "":
		return "101", nil
	case "1w":
		return "102", nil
	default:
		return "", fmt.Errorf("unsupported index interval: %s", interval)
	}
}

func parseEastmoneyKlineTime(raw string) (int64, error) {
	layouts := []string{"2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("unsupported time: %s", raw)
}

func setEastmoneyHeaders(req *http.Request, def IndexDef) {
	req.Header.Set("Referer", "https://quote.eastmoney.com/unify/r/"+def.EastmoneySecID)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
}
