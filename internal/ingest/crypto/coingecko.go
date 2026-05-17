package crypto

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

var coinGeckoMarketsURL = "https://api.coingecko.com/api/v3/coins/markets"

var CoinGeckoIDs = map[string]string{
	"ADA":  "cardano",
	"BNB":  "binancecoin",
	"BTC":  "bitcoin",
	"DOGE": "dogecoin",
	"ETH":  "ethereum",
	"FIL":  "filecoin",
	"LTC":  "litecoin",
	"SOL":  "solana",
	"XRP":  "ripple",
}

// FetchMarketMetadata loads market cap, rank, icon and 24h volume for symbols.
func FetchMarketMetadata(client *http.Client, symbols []string) ([]store.Quote, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ids, symbolsByID := resolveIDs(symbols)
	if len(ids) == 0 {
		return nil, fmt.Errorf("coingecko: no supported symbols")
	}

	q := url.Values{}
	q.Set("vs_currency", "usd")
	q.Set("ids", strings.Join(ids, ","))
	q.Set("sparkline", "false")

	req, err := http.NewRequest(http.MethodGet, coinGeckoMarketsURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coingecko markets request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko markets http %d", resp.StatusCode)
	}

	var rows []coinMarket
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("coingecko markets parse: %w", err)
	}

	now := time.Now().UTC()
	out := make([]store.Quote, 0, len(rows))
	for _, row := range rows {
		sym := symbolsByID[row.ID]
		if sym == "" {
			sym = strings.ToUpper(row.Symbol)
		}
		if sym == "" {
			continue
		}
		out = append(out, store.Quote{
			Symbol:       sym,
			Rank:         row.MarketCapRank,
			IconURL:      row.Image,
			MarketCapUsd: row.MarketCap,
			Volume24hUsd: row.TotalVolume,
			UpdatedAt:    now,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("coingecko markets: empty data")
	}
	return out, nil
}

func resolveIDs(symbols []string) ([]string, map[string]string) {
	ids := make([]string, 0, len(symbols))
	symbolsByID := make(map[string]string, len(symbols))
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		id := CoinGeckoIDs[symbol]
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		symbolsByID[id] = symbol
	}
	return ids, symbolsByID
}

type coinMarket struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Image         string  `json:"image"`
	MarketCapRank int     `json:"market_cap_rank"`
	MarketCap     float64 `json:"market_cap"`
	TotalVolume   float64 `json:"total_volume"`
}
