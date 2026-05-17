package derivatives

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/store"
)

var binanceLongShortURL = "https://fapi.binance.com/futures/data/globalLongShortAccountRatio"
var binanceTopLongShortURL = "https://fapi.binance.com/futures/data/topLongShortPositionRatio"
var binancePremiumIndexURL = "https://fapi.binance.com/fapi/v1/premiumIndex"
var binanceOpenInterestHistURL = "https://fapi.binance.com/futures/data/openInterestHist"
var binanceTakerBuySellURL = "https://fapi.binance.com/futures/data/takerlongshortRatio"

const AllLiquidationsStreamURL = "wss://fstream.binance.com/ws/!forceOrder@arr"

// FetchGlobalLongShort loads the latest Binance futures global long/short account ratio.
func FetchGlobalLongShort(client *http.Client, symbol string) (store.LongShortRatio, error) {
	return fetchLongShortRatio(client, binanceLongShortURL, symbol, "binance long-short")
}

// FetchTopLongShortPosition loads top trader position long/short ratio.
func FetchTopLongShortPosition(client *http.Client, symbol string) (store.LongShortRatio, error) {
	return fetchLongShortRatio(client, binanceTopLongShortURL, symbol, "binance top long-short")
}

func fetchLongShortRatio(client *http.Client, endpoint string, symbol string, label string) (store.LongShortRatio, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}

	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("period", "1h")
	q.Set("limit", "1")
	req, err := http.NewRequest(http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return store.LongShortRatio{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.LongShortRatio{}, fmt.Errorf("%s request: %w", label, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.LongShortRatio{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.LongShortRatio{}, fmt.Errorf("%s http %d", label, resp.StatusCode)
	}

	var rows []longShortRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return store.LongShortRatio{}, fmt.Errorf("%s parse: %w", label, err)
	}
	if len(rows) == 0 {
		return store.LongShortRatio{}, fmt.Errorf("%s: empty data", label)
	}
	row := rows[len(rows)-1]
	ratio, err := strconv.ParseFloat(row.LongShortRatio, 64)
	if err != nil {
		return store.LongShortRatio{}, fmt.Errorf("%s ratio: %w", label, err)
	}
	longAccount, err := strconv.ParseFloat(row.LongAccount, 64)
	if err != nil {
		return store.LongShortRatio{}, fmt.Errorf("%s long account: %w", label, err)
	}
	shortAccount, err := strconv.ParseFloat(row.ShortAccount, 64)
	if err != nil {
		return store.LongShortRatio{}, fmt.Errorf("%s short account: %w", label, err)
	}

	updatedAt := time.Now().UTC()
	if row.Timestamp > 0 {
		updatedAt = time.UnixMilli(row.Timestamp).UTC()
	}
	return store.LongShortRatio{
		Symbol:          symbol,
		Ratio:           ratio,
		LongAccountPct:  longAccount * 100,
		ShortAccountPct: shortAccount * 100,
		UpdatedAt:       updatedAt,
	}, nil
}

type longShortRow struct {
	Symbol         string `json:"symbol"`
	LongShortRatio string `json:"longShortRatio"`
	LongAccount    string `json:"longAccount"`
	ShortAccount   string `json:"shortAccount"`
	Timestamp      int64  `json:"timestamp"`
}

// FetchFunding loads the current funding rate from Binance USD-M futures mark price endpoint.
func FetchFunding(client *http.Client, symbol string) (store.FundingRate, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	req, err := http.NewRequest(http.MethodGet, binancePremiumIndexURL+"?"+q.Encode(), nil)
	if err != nil {
		return store.FundingRate{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.FundingRate{}, fmt.Errorf("binance funding request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.FundingRate{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.FundingRate{}, fmt.Errorf("binance funding http %d", resp.StatusCode)
	}

	var row premiumIndexRow
	if err := json.Unmarshal(body, &row); err != nil {
		return store.FundingRate{}, fmt.Errorf("binance funding parse: %w", err)
	}
	rate, err := strconv.ParseFloat(row.LastFundingRate, 64)
	if err != nil {
		return store.FundingRate{}, fmt.Errorf("binance funding rate: %w", err)
	}
	updatedAt := time.Now().UTC()
	if row.Time > 0 {
		updatedAt = time.UnixMilli(row.Time).UTC()
	}
	nextFunding := time.Time{}
	if row.NextFundingTime > 0 {
		nextFunding = time.UnixMilli(row.NextFundingTime).UTC()
	}
	return store.FundingRate{
		Symbol:          symbol,
		Rate:            rate,
		NextFundingTime: nextFunding,
		UpdatedAt:       updatedAt,
	}, nil
}

// FetchOpenInterest loads open interest value and recent change using 1h statistics.
func FetchOpenInterest(client *http.Client, symbol string) (store.OpenInterest, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("period", "1h")
	q.Set("limit", "2")
	req, err := http.NewRequest(http.MethodGet, binanceOpenInterestHistURL+"?"+q.Encode(), nil)
	if err != nil {
		return store.OpenInterest{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.OpenInterest{}, fmt.Errorf("binance open interest request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.OpenInterest{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.OpenInterest{}, fmt.Errorf("binance open interest http %d", resp.StatusCode)
	}

	var rows []openInterestHistRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return store.OpenInterest{}, fmt.Errorf("binance open interest parse: %w", err)
	}
	if len(rows) == 0 {
		return store.OpenInterest{}, fmt.Errorf("binance open interest: empty data")
	}
	last := rows[len(rows)-1]
	value, err := strconv.ParseFloat(last.SumOpenInterestValue, 64)
	if err != nil {
		return store.OpenInterest{}, fmt.Errorf("binance open interest value: %w", err)
	}
	change := 0.0
	if len(rows) >= 2 {
		prev, _ := strconv.ParseFloat(rows[len(rows)-2].SumOpenInterestValue, 64)
		if prev > 0 {
			change = (value - prev) / prev * 100
		}
	}
	updatedAt := time.Now().UTC()
	if last.Timestamp > 0 {
		updatedAt = time.UnixMilli(int64(last.Timestamp)).UTC()
	}
	return store.OpenInterest{
		Symbol:    symbol,
		ValueUsd:  value,
		ChangePct: change,
		UpdatedAt: updatedAt,
	}, nil
}

// FetchTakerBuySell loads taker buy/sell volume ratio from Binance futures.
func FetchTakerBuySell(client *http.Client, symbol string) (store.TakerBuySell, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("period", "1h")
	q.Set("limit", "1")
	req, err := http.NewRequest(http.MethodGet, binanceTakerBuySellURL+"?"+q.Encode(), nil)
	if err != nil {
		return store.TakerBuySell{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "marketpulse-marketd/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy/sell request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return store.TakerBuySell{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy/sell http %d", resp.StatusCode)
	}

	var rows []takerBuySellRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy/sell parse: %w", err)
	}
	if len(rows) == 0 {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy/sell: empty data")
	}
	row := rows[len(rows)-1]
	ratio, err := strconv.ParseFloat(row.BuySellRatio, 64)
	if err != nil {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy/sell ratio: %w", err)
	}
	buyVol, err := strconv.ParseFloat(row.BuyVol, 64)
	if err != nil {
		return store.TakerBuySell{}, fmt.Errorf("binance taker buy volume: %w", err)
	}
	sellVol, err := strconv.ParseFloat(row.SellVol, 64)
	if err != nil {
		return store.TakerBuySell{}, fmt.Errorf("binance taker sell volume: %w", err)
	}
	updatedAt := time.Now().UTC()
	if row.Timestamp > 0 {
		updatedAt = time.UnixMilli(int64(row.Timestamp)).UTC()
	}
	return store.TakerBuySell{
		Symbol:    symbol,
		Ratio:     ratio,
		BuyVol:    buyVol,
		SellVol:   sellVol,
		UpdatedAt: updatedAt,
	}, nil
}

// LiquidationOrder is a normalized force-liquidation event.
type LiquidationOrder struct {
	Symbol    string
	Side      string
	Notional  float64
	EventTime time.Time
}

// RunAllLiquidations connects to the all-market liquidation stream until ctx ends.
func RunAllLiquidations(ctx context.Context, streamURL string, onConnect func(), onOrder func(LiquidationOrder)) error {
	if streamURL == "" {
		streamURL = AllLiquidationsStreamURL
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, streamURL, http.Header{})
	if err != nil {
		return fmt.Errorf("binance liquidations dial: %w", err)
	}
	defer conn.Close()
	if onConnect != nil {
		onConnect()
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		case <-done:
		}
	}()
	defer close(done)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("binance liquidations read: %w", err)
		}
		order, ok := ParseLiquidationMessage(data)
		if !ok {
			continue
		}
		onOrder(order)
	}
}

// ParseLiquidationMessage normalizes Binance forceOrder payloads.
func ParseLiquidationMessage(data []byte) (LiquidationOrder, bool) {
	var ev liquidationEvent
	if err := json.Unmarshal(data, &ev); err != nil || ev.Order.Symbol == "" {
		return LiquidationOrder{}, false
	}
	qty, err := strconv.ParseFloat(ev.Order.AccumulatedQty, 64)
	if err != nil || qty <= 0 {
		qty, _ = strconv.ParseFloat(ev.Order.OriginalQty, 64)
	}
	price, err := strconv.ParseFloat(ev.Order.AveragePrice, 64)
	if err != nil || price <= 0 {
		price, _ = strconv.ParseFloat(ev.Order.Price, 64)
	}
	if qty <= 0 || price <= 0 {
		return LiquidationOrder{}, false
	}
	eventTime := time.Now().UTC()
	if ev.EventTime > 0 {
		eventTime = time.UnixMilli(ev.EventTime).UTC()
	}
	return LiquidationOrder{
		Symbol:    ev.Order.Symbol,
		Side:      ev.Order.Side,
		Notional:  qty * price,
		EventTime: eventTime,
	}, true
}

type premiumIndexRow struct {
	Symbol          string `json:"symbol"`
	LastFundingRate string `json:"lastFundingRate"`
	NextFundingTime int64  `json:"nextFundingTime"`
	Time            int64  `json:"time"`
}

type openInterestHistRow struct {
	Symbol               string    `json:"symbol"`
	SumOpenInterest      string    `json:"sumOpenInterest"`
	SumOpenInterestValue string    `json:"sumOpenInterestValue"`
	Timestamp            flexInt64 `json:"timestamp"`
}

type takerBuySellRow struct {
	BuySellRatio string    `json:"buySellRatio"`
	BuyVol       string    `json:"buyVol"`
	SellVol      string    `json:"sellVol"`
	Timestamp    flexInt64 `json:"timestamp"`
}

type liquidationEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Order     struct {
		Symbol         string `json:"s"`
		Side           string `json:"S"`
		OriginalQty    string `json:"q"`
		Price          string `json:"p"`
		AveragePrice   string `json:"ap"`
		AccumulatedQty string `json:"z"`
	} `json:"o"`
}

type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexInt64(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt64(parsed)
	return nil
}
