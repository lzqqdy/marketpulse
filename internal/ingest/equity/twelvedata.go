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

	"github.com/lzqqdy/marketpulse/internal/store"
)

const twelveDataQuoteBase = "https://api.twelvedata.com/quote"

type twelveQuote struct {
	Symbol        string `json:"symbol"`
	Close         string `json:"close"`
	PreviousClose string `json:"previous_close"`
	PercentChange string `json:"percent_change"`
	Datetime      string `json:"datetime"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	Code          int    `json:"code"`
}

// FetchTwelveDataQuotes loads quotes via Twelve Data. API key is required.
func FetchTwelveDataQuotes(client *http.Client, defs []IndexDef, apiKey string) (map[string]store.IndexQuote, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("twelvedata: missing api key")
	}
	if client == nil {
		client = http.DefaultClient
	}
	symbols := make([]string, 0, len(defs))
	bySymbol := make(map[string]IndexDef, len(defs))
	for _, def := range defs {
		if strings.TrimSpace(def.TwelveSymbol) == "" {
			continue
		}
		symbols = append(symbols, def.TwelveSymbol)
		bySymbol[strings.ToUpper(def.TwelveSymbol)] = def
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("twelvedata: no symbols")
	}

	q := url.Values{}
	q.Set("symbol", strings.Join(symbols, ","))
	q.Set("apikey", apiKey)
	req, err := http.NewRequest(http.MethodGet, twelveDataQuoteBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "twelvedata", "endpoint", "quote", "symbols", len(symbols), "err", err)
		return nil, fmt.Errorf("twelvedata request: %w", err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "twelvedata", "endpoint", "quote", "symbols", len(symbols), "status", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("twelvedata http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twelvedata http %d", resp.StatusCode)
	}
	return parseTwelveQuotes(body, bySymbol, time.Now().UTC())
}

func parseTwelveQuotes(body []byte, bySymbol map[string]IndexDef, now time.Time) (map[string]store.IndexQuote, error) {
	var single twelveQuote
	if err := json.Unmarshal(body, &single); err == nil && single.Symbol != "" {
		out := make(map[string]store.IndexQuote, 1)
		row, err := twelveQuoteToIndex(single, bySymbol, now)
		if err != nil {
			return nil, err
		}
		out[row.ID] = row
		return out, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("twelvedata parse: %w", err)
	}
	out := make(map[string]store.IndexQuote, len(raw))
	var firstErr error
	for key, data := range raw {
		if strings.EqualFold(key, "status") || strings.EqualFold(key, "message") || strings.EqualFold(key, "code") {
			continue
		}
		var q twelveQuote
		if err := json.Unmarshal(data, &q); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if q.Symbol == "" {
			q.Symbol = key
		}
		row, err := twelveQuoteToIndex(q, bySymbol, now)
		if err != nil {
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
	if len(out) == 0 {
		return nil, fmt.Errorf("twelvedata: empty result")
	}
	return out, firstErr
}

func twelveQuoteToIndex(q twelveQuote, bySymbol map[string]IndexDef, now time.Time) (store.IndexQuote, error) {
	if strings.EqualFold(q.Status, "error") {
		return store.IndexQuote{}, fmt.Errorf("twelvedata %s: %s", q.Symbol, q.Message)
	}
	def, ok := bySymbol[strings.ToUpper(q.Symbol)]
	if !ok {
		return store.IndexQuote{}, fmt.Errorf("twelvedata %s: unknown symbol", q.Symbol)
	}
	price, err := strconv.ParseFloat(q.Close, 64)
	if err != nil {
		return store.IndexQuote{}, fmt.Errorf("twelvedata %s: empty quote", q.Symbol)
	}
	if err := validatePrice(def, price); err != nil {
		return store.IndexQuote{}, fmt.Errorf("twelvedata %s: %w", q.Symbol, err)
	}
	chg, _ := strconv.ParseFloat(q.PercentChange, 64)
	if chg == 0 {
		prev, _ := strconv.ParseFloat(q.PreviousClose, 64)
		if prev > 0 {
			chg = (price - prev) / prev * 100
		}
	}
	updatedAt := now
	if q.Datetime != "" {
		if t, err := parseTwelveTime(q.Datetime); err == nil {
			updatedAt = t
		}
	}
	return store.IndexQuote{
		ID:        def.ID,
		Name:      def.Name,
		Price:     price,
		ChangePct: chg,
		Source:    "twelvedata",
		FetchedAt: now,
		UpdatedAt: updatedAt,
	}, nil
}

func parseTwelveTime(raw string) (time.Time, error) {
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time: %s", raw)
}
