package otc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var apiURL = "https://www.okx.com/v3/c2c/otc-ticker/quotedPrice"

// FetchUSDTCNY returns OKX C2C best USDT/CNY price.
func FetchUSDTCNY(client *http.Client) (float64, error) {
	if client == nil {
		client = http.DefaultClient
	}
	q := url.Values{}
	q.Set("baseCurrency", "USDT")
	q.Set("quoteCurrency", "CNY")
	q.Set("side", "buy")
	q.Set("amount", "10000")
	q.Set("standard", "1")

	req, err := http.NewRequest(http.MethodGet, apiURL+"?"+q.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("okx otc request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("okx otc http %d: %s", resp.StatusCode, truncate(string(body), 160))
	}

	var parsed struct {
		Code int `json:"code"`
		Data []struct {
			BestOption bool   `json:"bestOption"`
			Price      string `json:"price"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("okx otc parse: %w", err)
	}
	for _, row := range parsed.Data {
		if !row.BestOption {
			continue
		}
		p, err := strconv.ParseFloat(row.Price, 64)
		if err != nil || p <= 0 {
			return 0, fmt.Errorf("okx otc invalid price %q", row.Price)
		}
		return p, nil
	}
	return 0, fmt.Errorf("okx otc: no bestOption row")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// NowUTC is used by tests and callers.
func NowUTC() time.Time { return time.Now().UTC() }
