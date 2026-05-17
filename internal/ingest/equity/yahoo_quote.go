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

const quoteBase = "https://query1.finance.yahoo.com/v7/finance/quote"

type yahooQuoteResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol                     string  `json:"symbol"`
			RegularMarketPrice         float64 `json:"regularMarketPrice"`
			RegularMarketChangePct     float64 `json:"regularMarketChangePercent"`
			RegularMarketTime          int64   `json:"regularMarketTime"`
			RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"quoteResponse"`
}

// FetchYahooQuotes loads a batch of index quotes from Yahoo quote API.
func FetchYahooQuotes(client *http.Client, defs []IndexDef) (map[string]store.IndexQuote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(defs) == 0 {
		defs = DefaultIndices
	}
	symbols := make([]string, 0, len(defs))
	bySymbol := make(map[string]IndexDef, len(defs))
	for _, def := range defs {
		if def.Symbol == "" {
			continue
		}
		symbols = append(symbols, def.Symbol)
		bySymbol[strings.ToUpper(def.Symbol)] = def
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("yahoo quote: no symbols")
	}

	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	req, err := http.NewRequest(http.MethodGet, quoteBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MarketPulse/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("equity http request failed", "provider", "yahoo", "endpoint", "quote", "symbols", len(symbols), "err", err)
		return nil, fmt.Errorf("yahoo quote request: %w", err)
	}
	defer resp.Body.Close()
	slog.Info("equity http response", "provider", "yahoo", "endpoint", "quote", "symbols", len(symbols), "status", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("yahoo quote http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo quote http %d", resp.StatusCode)
	}

	var parsed yahooQuoteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo quote parse: %w", err)
	}
	now := time.Now().UTC()
	out := make(map[string]store.IndexQuote, len(parsed.QuoteResponse.Result))
	for _, row := range parsed.QuoteResponse.Result {
		def, ok := bySymbol[strings.ToUpper(row.Symbol)]
		if !ok || validatePrice(def, row.RegularMarketPrice) != nil {
			continue
		}
		updatedAt := now
		if row.RegularMarketTime > 0 {
			updatedAt = time.Unix(row.RegularMarketTime, 0).UTC()
		}
		chg := row.RegularMarketChangePct
		if chg == 0 && row.RegularMarketPreviousClose > 0 {
			chg = (row.RegularMarketPrice - row.RegularMarketPreviousClose) / row.RegularMarketPreviousClose * 100
		}
		out[def.ID] = store.IndexQuote{
			ID:        def.ID,
			Name:      def.Name,
			Price:     row.RegularMarketPrice,
			ChangePct: chg,
			Source:    "yahoo",
			FetchedAt: now,
			UpdatedAt: updatedAt,
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("yahoo quote: empty result")
	}
	slog.Info("equity quote parsed", "provider", "yahoo", "requested", len(defs), "parsed", len(out))
	if len(out) < len(defs) {
		return out, fmt.Errorf("yahoo quote: partial result %d/%d", len(out), len(defs))
	}
	return out, nil
}
