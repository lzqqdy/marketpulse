package equity

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

const finnhubQuoteBase = "https://finnhub.io/api/v1/quote"

type finnhubQuote struct {
	Current       float64 `json:"c"`
	ChangePct     float64 `json:"dp"`
	PreviousClose float64 `json:"pc"`
	Time          int64   `json:"t"`
}

// FetchFinnhubQuotes loads index quotes via Finnhub. API key is required.
func FetchFinnhubQuotes(client *http.Client, defs []IndexDef, apiKey string) (map[string]store.IndexQuote, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("finnhub: missing api key")
	}
	if client == nil {
		client = http.DefaultClient
	}
	now := time.Now().UTC()
	out := make(map[string]store.IndexQuote, len(defs))
	var firstErr error
	for i, def := range defs {
		if strings.TrimSpace(def.FinnhubSymbol) == "" {
			continue
		}
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		q, err := fetchFinnhubOne(client, def, apiKey, now)
		if err != nil {
			slog.Warn("equity symbol fetch failed", "provider", "finnhub", "id", def.ID, "symbol", def.FinnhubSymbol, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			if IsRateLimitErr(err) {
				break
			}
			continue
		}
		out[def.ID] = q
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, firstErr
}

func fetchFinnhubOne(client *http.Client, def IndexDef, apiKey string, now time.Time) (store.IndexQuote, error) {
	q := url.Values{}
	q.Set("symbol", def.FinnhubSymbol)
	q.Set("token", apiKey)
	req, err := http.NewRequest(http.MethodGet, finnhubQuoteBase+"?"+q.Encode(), nil)
	if err != nil {
		return store.IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "finnhub", "id", def.ID, "symbol", def.FinnhubSymbol, "err", err)
		return store.IndexQuote{}, fmt.Errorf("%s finnhub request: %w", def.ID, err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "finnhub", "id", def.ID, "symbol", def.FinnhubSymbol, "status", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.IndexQuote{}, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return store.IndexQuote{}, fmt.Errorf("%s finnhub http %d", def.ID, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return store.IndexQuote{}, fmt.Errorf("%s finnhub http %d", def.ID, resp.StatusCode)
	}

	var parsed finnhubQuote
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s finnhub parse: %w", def.ID, err)
	}
	if err := validatePrice(def, parsed.Current); err != nil {
		return store.IndexQuote{}, fmt.Errorf("%s finnhub: %w", def.ID, err)
	}
	chg := parsed.ChangePct
	if chg == 0 && parsed.PreviousClose > 0 {
		chg = (parsed.Current - parsed.PreviousClose) / parsed.PreviousClose * 100
	}
	updatedAt := now
	if parsed.Time > 0 {
		updatedAt = time.Unix(parsed.Time, 0).UTC()
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     parsed.Current,
		ChangePct: chg,
		Source:    "finnhub",
		FetchedAt: now,
		UpdatedAt: updatedAt,
	}, nil
}
