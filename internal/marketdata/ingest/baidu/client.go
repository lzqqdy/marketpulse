package baidu

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
)

func normalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if strings.TrimSpace(cfg.WSURL) == "" {
		cfg.WSURL = DefaultWSURL
	}
	if cfg.WSReconnectMax <= 0 {
		cfg.WSReconnectMax = 5
	}
	if cfg.WSReconnectDelay <= 0 {
		cfg.WSReconnectDelay = 3
	}
	if cfg.WSPatchInterval <= 0 {
		cfg.WSPatchInterval = 60
	}
	return cfg
}

func getJSON(client *http.Client, baseURL, path string, query url.Values) (APIResponse, error) {
	bases := requestBases(baseURL)
	var lastErr error
	for _, base := range bases {
		envelope, err := getJSONOnce(client, base, path, query)
		if err == nil {
			return envelope, nil
		}
		lastErr = err
		if !isRetryableHTTP(err) {
			return APIResponse{}, err
		}
	}
	if lastErr != nil {
		return APIResponse{}, lastErr
	}
	return APIResponse{}, fmt.Errorf("baidu: no base url")
}

func requestBases(primary string) []string {
	primary = strings.TrimRight(strings.TrimSpace(primary), "/")
	if primary == "" {
		primary = DefaultBaseURL
	}
	if primary == DefaultCDNBase {
		return []string{primary}
	}
	return []string{primary, DefaultCDNBase}
}

func isRetryableHTTP(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "http 403") || strings.Contains(msg, "http 502") || strings.Contains(msg, "http 503")
}

func getJSONOnce(client *http.Client, baseURL, path string, query url.Values) (APIResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	reqURL := baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return APIResponse{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://finance.baidu.com/")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return APIResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return APIResponse{}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return APIResponse{}, fmt.Errorf("baidu http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return APIResponse{}, fmt.Errorf("baidu http %d: %s", resp.StatusCode, truncate(string(body), 120))
	}
	var envelope APIResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return APIResponse{}, fmt.Errorf("baidu parse: %w", err)
	}
	if !envelope.OK() {
		return envelope, fmt.Errorf("baidu api: %s", strings.TrimSpace(envelope.ResultMsg))
	}
	return envelope, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func parseBaiduPercent(raw string) float64 {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "%")
	raw = strings.TrimPrefix(raw, "+")
	v, _ := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	return v
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
