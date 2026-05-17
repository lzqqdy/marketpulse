package macro

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/lzqqdy/marketpulse/internal/store"
)

const (
	fngURL      = "https://api.alternative.me/fng/?limit=1"
	coingeckoURL = "https://api.coingecko.com/api/v3/global"
)

// Fetch merges fear/greed and CoinGecko global into MacroSnapshot.
func Fetch(client *http.Client) (store.MacroSnapshot, error) {
	if client == nil {
		client = http.DefaultClient
	}
	fng, err := fetchFearGreed(client)
	if err != nil {
		return store.MacroSnapshot{}, err
	}
	global, err := fetchCoinGeckoGlobal(client)
	if err != nil {
		return store.MacroSnapshot{}, err
	}
	global.FearGreed = fng
	return global, nil
}

func fetchFearGreed(client *http.Client) (store.FearGreed, error) {
	resp, err := client.Get(fngURL)
	if err != nil {
		return store.FearGreed{}, fmt.Errorf("fng request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.FearGreed{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.FearGreed{}, fmt.Errorf("fng http %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			Value               string `json:"value"`
			ValueClassification string `json:"value_classification"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.FearGreed{}, err
	}
	if len(parsed.Data) == 0 {
		return store.FearGreed{}, fmt.Errorf("fng: empty data")
	}
	v, _ := strconv.Atoi(parsed.Data[0].Value)
	label := parsed.Data[0].ValueClassification
	if label == "" {
		label = classifyFNG(v)
	}
	return store.FearGreed{Value: v, Label: label}, nil
}

func fetchCoinGeckoGlobal(client *http.Client) (store.MacroSnapshot, error) {
	req, err := http.NewRequest(http.MethodGet, coingeckoURL, nil)
	if err != nil {
		return store.MacroSnapshot{}, err
	}
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.MacroSnapshot{}, fmt.Errorf("coingecko request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.MacroSnapshot{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.MacroSnapshot{}, fmt.Errorf("coingecko http %d", resp.StatusCode)
	}

	var parsed struct {
		Data struct {
			TotalMarketCap struct {
				USD float64 `json:"usd"`
			} `json:"total_market_cap"`
			TotalVolume struct {
				USD float64 `json:"usd"`
			} `json:"total_volume"`
			MarketCapChangePercentage24hUSD float64 `json:"market_cap_change_percentage_24h_usd"`
			MarketCapPercentage             struct {
				BTC float64 `json:"btc"`
				ETH float64 `json:"eth"`
			} `json:"market_cap_percentage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return store.MacroSnapshot{}, err
	}
	d := parsed.Data
	return store.MacroSnapshot{
		TotalMarketCapUsd:          d.TotalMarketCap.USD,
		TotalVolume24hUsd:          d.TotalVolume.USD,
		TotalMarketCapChange24hPct: d.MarketCapChangePercentage24hUSD,
		BTCDominancePct:            d.MarketCapPercentage.BTC,
		ETHDominancePct:            d.MarketCapPercentage.ETH,
	}, nil
}

func classifyFNG(v int) string {
	switch {
	case v <= 24:
		return "Extreme Fear"
	case v <= 44:
		return "Fear"
	case v <= 55:
		return "Neutral"
	case v <= 75:
		return "Greed"
	default:
		return "Extreme Greed"
	}
}

// NowUTC for tests.
func NowUTC() time.Time { return time.Now().UTC() }
