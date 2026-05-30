package bitget

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

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

const (
	baseURL     = "https://api.bitget.com"
	wsURL       = "wss://ws.bitget.com/v2/ws/public"
	tickersPath = "/api/v2/mix/market/tickers"
	candlesPath = "/api/v2/mix/market/history-candles"
)

type ResolvedItem struct {
	Item       config.AlphaItem
	Category   string
	BaseSymbol string
	Symbol     string
}

type Ticker struct {
	Symbol       string
	Price        float64
	Change24hPct float64
	Volume       float64
	MarkPrice    float64
	IndexPrice   float64
	FundingRate  float64
	UpdatedAt    time.Time
}

type tickerRaw struct {
	Symbol      string `json:"symbol"`
	LastPr      string `json:"lastPr"`
	Change24h   string `json:"change24h"`
	BaseVolume  string `json:"baseVolume"`
	USDTVolume  string `json:"usdtVolume"`
	QuoteVolume string `json:"quoteVolume"`
	MarkPrice   string `json:"markPrice"`
	IndexPrice  string `json:"indexPrice"`
	FundingRate string `json:"fundingRate"`
	TS          string `json:"ts"`
}

type wsMessage struct {
	Event  string          `json:"event"`
	Action string          `json:"action"`
	Arg    wsArg           `json:"arg"`
	Data   json.RawMessage `json:"data"`
}

type wsArg struct {
	InstType string `json:"instType"`
	Channel  string `json:"channel"`
	InstID   string `json:"instId"`
}

func ResolveItems(client *http.Client, indices []config.AlphaItem, stocks []config.AlphaItem, quoteAsset, productType string) ([]ResolvedItem, []config.AlphaItem, error) {
	tickers, err := FetchTickers(client, productType)
	if err != nil {
		return nil, append([]config.AlphaItem{}, append(indices, stocks...)...), err
	}
	available := make(map[string]struct{}, len(tickers))
	for symbol := range tickers {
		available[symbol] = struct{}{}
	}
	out, missing := resolveItemsFromAvailable(indices, stocks, quoteAsset, available)
	return out, missing, nil
}

func resolveItemsFromAvailable(indices []config.AlphaItem, stocks []config.AlphaItem, quoteAsset string, available map[string]struct{}) ([]ResolvedItem, []config.AlphaItem) {
	out := make([]ResolvedItem, 0, len(indices)+len(stocks))
	missing := make([]config.AlphaItem, 0)
	add := func(items []config.AlphaItem, category string) {
		for _, item := range items {
			symbol := normalizeSymbol(item.Symbol, quoteAsset)
			base := strings.TrimSuffix(symbol, quoteAsset)
			if base == "" {
				base = strings.ToUpper(item.ID)
			}
			if _, ok := available[symbol]; !ok {
				missing = append(missing, item)
				continue
			}
			out = append(out, ResolvedItem{
				Item:       item,
				Category:   category,
				BaseSymbol: base,
				Symbol:     symbol,
			})
		}
	}
	add(indices, "index")
	add(stocks, "stock")
	return out, missing
}

func FetchTickers(client *http.Client, productType string) (map[string]Ticker, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var body struct {
		Code string      `json:"code"`
		Msg  string      `json:"msg"`
		Data []tickerRaw `json:"data"`
	}
	if err := getJSON(client, tickersPath, url.Values{"productType": {productType}}, &body); err != nil {
		return nil, err
	}
	if body.Code != "" && body.Code != "00000" {
		return nil, fmt.Errorf("bitget tickers code %s: %s", body.Code, body.Msg)
	}
	out := parseTickers(body.Data)
	if len(out) == 0 {
		return nil, fmt.Errorf("bitget tickers: no rows")
	}
	return out, nil
}

func FetchKlines(client *http.Client, symbol, productType, interval string, limit int) ([]binance.Candle, error) {
	if client == nil {
		client = http.DefaultClient
	}
	granularity, err := Granularity(interval)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 200 {
		limit = 200
	}
	var body struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := getJSON(client, candlesPath, url.Values{
		"symbol":      {strings.ToUpper(strings.TrimSpace(symbol))},
		"productType": {productType},
		"granularity": {granularity},
		"limit":       {strconv.Itoa(limit)},
	}, &body); err != nil {
		return nil, err
	}
	if body.Code != "" && body.Code != "00000" {
		return nil, fmt.Errorf("bitget candles code %s: %s", body.Code, body.Msg)
	}
	candles := ParseCandleRows(body.Data)
	if len(candles) == 0 {
		return nil, fmt.Errorf("bitget candles %s: no candles", symbol)
	}
	return candles, nil
}

func StreamTicker(ctx context.Context, productType string, symbols []string, onTick func(Ticker)) error {
	args := make([]wsArg, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		args = append(args, wsArg{InstType: productType, Channel: "ticker", InstID: symbol})
	}
	if len(args) == 0 {
		return fmt.Errorf("bitget ticker ws: no symbols")
	}
	return stream(ctx, args, func(msg wsMessage) {
		if msg.Arg.Channel != "ticker" {
			return
		}
		var rows []tickerRaw
		if json.Unmarshal(msg.Data, &rows) != nil {
			return
		}
		for _, row := range rows {
			if tick, ok := ParseTicker(row); ok {
				onTick(tick)
			}
		}
	})
}

func StreamKline(ctx context.Context, productType, symbol, interval string, onCandle func(binance.Candle)) error {
	channel, err := CandleChannel(interval)
	if err != nil {
		return err
	}
	args := []wsArg{{InstType: productType, Channel: channel, InstID: strings.ToUpper(strings.TrimSpace(symbol))}}
	return stream(ctx, args, func(msg wsMessage) {
		if msg.Arg.Channel != channel {
			return
		}
		candles, ok := ParseWSCandles(msg.Data)
		if !ok {
			return
		}
		for _, candle := range candles {
			onCandle(candle)
		}
	})
}

func ParseTicker(row tickerRaw) (Ticker, bool) {
	price := parseFloat(row.LastPr)
	if price <= 0 {
		return Ticker{}, false
	}
	updatedAt := time.Now().UTC()
	if ts := parseInt(row.TS); ts > 0 {
		updatedAt = time.UnixMilli(ts).UTC()
	}
	return Ticker{
		Symbol:       strings.ToUpper(strings.TrimSpace(row.Symbol)),
		Price:        price,
		Change24hPct: normalizeChangePct(parseFloat(row.Change24h)),
		Volume:       firstPositive(parseFloat(row.USDTVolume), parseFloat(row.QuoteVolume), parseFloat(row.BaseVolume)),
		MarkPrice:    parseFloat(row.MarkPrice),
		IndexPrice:   parseFloat(row.IndexPrice),
		FundingRate:  parseFloat(row.FundingRate),
		UpdatedAt:    updatedAt,
	}, true
}

func ParseCandleRows(rows [][]string) []binance.Candle {
	out := make([]binance.Candle, 0, len(rows))
	for _, row := range rows {
		if len(row) < 6 {
			continue
		}
		ts := parseInt(row[0])
		open := parseFloat(row[1])
		high := parseFloat(row[2])
		low := parseFloat(row[3])
		closep := parseFloat(row[4])
		if ts <= 0 || open <= 0 || high <= 0 || low <= 0 || closep <= 0 {
			continue
		}
		quoteVolume := 0.0
		if len(row) > 6 {
			quoteVolume = parseFloat(row[6])
		}
		out = append(out, binance.Candle{
			Time:        ts / 1000,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       closep,
			Volume:      parseFloat(row[5]),
			QuoteVolume: quoteVolume,
			Closed:      true,
		})
	}
	return out
}

func ParseWSMessage(data []byte) ([]Ticker, []binance.Candle, bool) {
	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil || len(msg.Data) == 0 {
		return nil, nil, false
	}
	switch {
	case msg.Arg.Channel == "ticker":
		var rows []tickerRaw
		if err := json.Unmarshal(msg.Data, &rows); err != nil {
			return nil, nil, false
		}
		tickers := make([]Ticker, 0, len(rows))
		for _, row := range rows {
			if tick, ok := ParseTicker(row); ok {
				tickers = append(tickers, tick)
			}
		}
		return tickers, nil, len(tickers) > 0
	case strings.HasPrefix(msg.Arg.Channel, "candle"):
		candles, ok := ParseWSCandles(msg.Data)
		return nil, candles, ok
	default:
		return nil, nil, false
	}
}

func ParseWSCandles(raw json.RawMessage) ([]binance.Candle, bool) {
	var rows [][]string
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, false
	}
	candles := ParseCandleRows(rows)
	for i := range candles {
		candles[i].Closed = false
	}
	return candles, len(candles) > 0
}

func Granularity(interval string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "1m":
		return "1m", nil
	case "5m":
		return "5m", nil
	case "15m":
		return "15m", nil
	case "1h":
		return "1H", nil
	case "4h":
		return "4H", nil
	case "1d":
		return "1D", nil
	case "1w":
		return "1W", nil
	default:
		return "", fmt.Errorf("unsupported bitget interval: %s", interval)
	}
}

func CandleChannel(interval string) (string, error) {
	g, err := Granularity(interval)
	if err != nil {
		return "", err
	}
	switch g {
	case "1m", "5m", "1H":
	default:
		return "", fmt.Errorf("unsupported bitget ws interval: %s", interval)
	}
	return "candle" + g, nil
}

func normalizeSymbol(symbol, quoteAsset string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	quoteAsset = strings.ToUpper(strings.TrimSpace(quoteAsset))
	if symbol == "" {
		return ""
	}
	if quoteAsset != "" && !strings.HasSuffix(symbol, quoteAsset) {
		symbol += quoteAsset
	}
	return symbol
}

func parseTickers(rows []tickerRaw) map[string]Ticker {
	out := make(map[string]Ticker, len(rows))
	for _, row := range rows {
		if tick, ok := ParseTicker(row); ok {
			out[tick.Symbol] = tick
		}
	}
	return out
}

func stream(ctx context.Context, args []wsArg, onMessage func(wsMessage)) error {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsURL, http.Header{})
	if err != nil {
		return fmt.Errorf("bitget ws dial: %w", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"op": "subscribe", "args": args}); err != nil {
		return fmt.Errorf("bitget ws subscribe: %w", err)
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
	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
					return
				}
			}
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("bitget ws read: %w", err)
		}
		if string(data) == "pong" {
			continue
		}
		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil || len(msg.Data) == 0 {
			continue
		}
		onMessage(msg)
	}
}

func getJSON(client *http.Client, path string, q url.Values, out any) error {
	u := baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MarketPulse/1.0")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bitget http %d: %s", resp.StatusCode, truncate(string(data), 180))
	}
	return json.Unmarshal(data, out)
}

func normalizeChangePct(v float64) float64 {
	if v > -1 && v < 1 {
		return v * 100
	}
	return v
}

func firstPositive(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func parseFloat(s string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return n
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
