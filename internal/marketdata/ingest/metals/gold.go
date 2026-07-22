// Package metals fetches domestic gold quotes (RMB/gram).
package metals

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

const (
	domesticGoldID   = "sge-au9999" // stable public id (legacy); UI keeps 国内金价
	domesticGoldName = "国内金价"

	eastmoneyGoldURL = "https://push2.eastmoney.com/api/qt/stock/get"
	eastmoneySecID   = "118.AU9999"

	sinaGoldURL    = "https://hq.sinajs.cn/?list=gds_AUTD"
	sinaGoldReferer = "https://finance.sina.com.cn"

	minGoldPrice = 100.0
	maxGoldPrice = 5000.0
)

var (
	// Overridable in tests.
	eastmoneyQuoteURL = eastmoneyGoldURL
	sinaQuoteURL      = sinaGoldURL
)

type eastmoneyResp struct {
	RC   int `json:"rc"`
	Data *struct {
		Code      string  `json:"f57"`
		Name      string  `json:"f58"`
		Price     float64 `json:"f43"`
		Open      float64 `json:"f46"`
		PrevClose float64 `json:"f60"`
		ChangePct float64 `json:"f170"`
		UpdatedAt int64   `json:"f86"`
	} `json:"data"`
}

// FetchAu9999 loads domestic gold RMB/gram quote.
// Primary: Eastmoney AU9999; fallback: Sina gold T+D (gds_AUTD).
func FetchAu9999(client *http.Client) (store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	q, err := FetchEastmoneyGold(client)
	if err == nil {
		return q, nil
	}
	fb, fbErr := FetchSinaGold(client)
	if fbErr == nil {
		return fb, nil
	}
	return store.IndexQuote{}, fmt.Errorf("domestic gold: eastmoney: %v; sina: %w", err, fbErr)
}

// FetchEastmoneyGold loads AU9999 quote from Eastmoney.
func FetchEastmoneyGold(client *http.Client) (store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	q := url.Values{}
	q.Set("fltt", "2")
	q.Set("secid", eastmoneySecID)
	q.Set("fields", "f43,f46,f57,f58,f60,f86,f169,f170")

	req, err := http.NewRequest(http.MethodGet, eastmoneyQuoteURL+"?"+q.Encode(), nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	resp, err := client.Do(req)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("http %d", resp.StatusCode)
	}

	var parsed eastmoneyResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.IndexQuote{}, fmt.Errorf("decode: %w", err)
	}
	if parsed.RC != 0 || parsed.Data == nil {
		return store.IndexQuote{}, fmt.Errorf("empty data rc=%d", parsed.RC)
	}
	price := parsed.Data.Price
	if !validGoldPrice(price) {
		return store.IndexQuote{}, fmt.Errorf("invalid price %v", price)
	}
	changePct := parsed.Data.ChangePct
	if changePct == 0 && parsed.Data.PrevClose > 0 {
		changePct = (price - parsed.Data.PrevClose) / parsed.Data.PrevClose * 100
	}
	updated := time.Now().UTC()
	if parsed.Data.UpdatedAt > 0 {
		updated = time.Unix(parsed.Data.UpdatedAt, 0).UTC()
	}
	return store.IndexQuote{
		ID:        domesticGoldID,
		Name:      domesticGoldName,
		Price:     price,
		ChangePct: changePct,
		Source:    "eastmoney",
		UpdatedAt: updated,
	}, nil
}

// FetchSinaGold loads gold T+D quote from Sina (gds_AUTD) as fallback.
func FetchSinaGold(client *http.Client) (store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, sinaQuoteURL, nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Referer", sinaGoldReferer)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("http %d", resp.StatusCode)
	}
	return parseSinaGold(string(body))
}

// parseSinaGold parses hq_str_gds_AUTD="price,...,high,low,time,...,prev?,...,date,name"
func parseSinaGold(raw string) (store.IndexQuote, error) {
	start := strings.Index(raw, `="`)
	end := strings.LastIndex(raw, `"`)
	if start < 0 || end <= start+2 {
		return store.IndexQuote{}, fmt.Errorf("empty payload")
	}
	payload := raw[start+2 : end]
	if payload == "" {
		return store.IndexQuote{}, fmt.Errorf("empty quote")
	}
	fields := strings.Split(payload, ",")
	if len(fields) < 6 {
		return store.IndexQuote{}, fmt.Errorf("too few fields")
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
	if err != nil || !validGoldPrice(price) {
		return store.IndexQuote{}, fmt.Errorf("invalid price %q", fields[0])
	}
	changePct := 0.0
	// Prefer explicit prev-close-like fields when present.
	for _, idx := range []int{8, 7} {
		if idx >= len(fields) {
			continue
		}
		prev, err := strconv.ParseFloat(strings.TrimSpace(fields[idx]), 64)
		if err != nil || prev <= 0 {
			continue
		}
		changePct = (price - prev) / prev * 100
		break
	}
	return store.IndexQuote{
		ID:        domesticGoldID,
		Name:      domesticGoldName,
		Price:     price,
		ChangePct: changePct,
		Source:    "sina",
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func validGoldPrice(v float64) bool {
	return v >= minGoldPrice && v <= maxGoldPrice
}
