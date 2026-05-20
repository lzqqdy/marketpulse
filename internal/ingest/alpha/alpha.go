package alpha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/binance"
	"github.com/lzqqdy/marketpulse/internal/config"
)

const (
	baseURL       = "https://www.binance.com"
	tickerPath    = "/bapi/defi/v1/public/alpha-trade/ticker"
	klinesPath    = "/bapi/defi/v1/public/alpha-trade/klines"
	exchangePath  = "/bapi/defi/v1/public/alpha-trade/get-exchange-info"
	tokenListPath = "/bapi/defi/v1/public/wallet-direct/buw/wallet/cex/alpha/all/token/list"
	WSURL         = "wss://nbstream.binance.com/w3w/wsa/stream"
)

var alphaSymbolPattern = regexp.MustCompile(`^ALPHA_[0-9]+[A-Z]+$`)

var resolveCache struct {
	sync.Mutex
	docs      []any
	expiresAt time.Time
}

type ResolvedItem struct {
	Item        config.AlphaItem
	Category    string
	BaseSymbol  string
	AlphaSymbol string
}

type Ticker struct {
	Symbol       string
	Price        float64
	Change24hPct float64
	Volume       float64
	UpdatedAt    time.Time
}

type combinedMessage struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type tickerEvent struct {
	EventType      string `json:"e"`
	EventTimeMs    int64  `json:"E"`
	Symbol         string `json:"s"`
	Close          string `json:"c"`
	PriceChangePct string `json:"P"`
	Open           string `json:"o"`
	Volume         string `json:"v"`
	QuoteVolume    string `json:"q"`
}

func ResolveItems(client *http.Client, indices []config.AlphaItem, stocks []config.AlphaItem, quoteAsset string) []ResolvedItem {
	if client == nil {
		client = http.DefaultClient
	}
	quoteAsset = strings.ToUpper(strings.TrimSpace(quoteAsset))
	docs := fetchResolveDocs(client)
	out := make([]ResolvedItem, 0, len(indices)+len(stocks))
	add := func(items []config.AlphaItem, category string) {
		for _, item := range items {
			base := strings.TrimSuffix(strings.ToUpper(item.Symbol), quoteAsset)
			if base == "" {
				base = strings.ToUpper(item.ID)
			}
			alphaSymbol := resolveAlphaSymbol(item, base, quoteAsset, docs)
			out = append(out, ResolvedItem{
				Item:        item,
				Category:    category,
				BaseSymbol:  base,
				AlphaSymbol: alphaSymbol,
			})
		}
	}
	add(indices, "index")
	add(stocks, "stock")
	return out
}

func FetchTicker(client *http.Client, symbol string) (Ticker, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var body any
	if err := getJSON(client, tickerPath, url.Values{"symbol": {symbol}}, &body); err != nil {
		return Ticker{}, err
	}
	data := unwrapData(body)
	price := firstFloat(data, "lastPrice", "price", "close", "c")
	if price <= 0 {
		return Ticker{}, fmt.Errorf("alpha ticker %s: missing price", symbol)
	}
	return Ticker{
		Symbol:       symbol,
		Price:        price,
		Change24hPct: firstFloat(data, "priceChangePercent", "priceChangePct", "changePercent", "change24hPct", "P"),
		Volume:       firstFloat(data, "volume", "quoteVolume", "v"),
		UpdatedAt:    time.Now().UTC(),
	}, nil
}

func RunTicker(ctx context.Context, items []ResolvedItem, onTick func(Ticker)) error {
	if len(items) == 0 {
		return fmt.Errorf("alpha ticker: no symbols")
	}
	streams := make([]string, 0, len(items))
	for _, item := range items {
		if !alphaSymbolPattern.MatchString(item.AlphaSymbol) {
			continue
		}
		streams = append(streams, strings.ToLower(item.AlphaSymbol)+"@ticker")
	}
	if len(streams) == 0 {
		return fmt.Errorf("alpha ticker: no resolved alpha symbols")
	}

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, WSURL, http.Header{})
	if err != nil {
		return fmt.Errorf("alpha ticker ws dial: %w", err)
	}
	defer conn.Close()

	const readWait = 90 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readWait))
	})

	sub := map[string]any{
		"method": "SUBSCRIBE",
		"params": streams,
		"id":     1,
	}
	if err := conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("alpha ticker ws subscribe: %w", err)
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

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
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
			return fmt.Errorf("alpha ticker ws read: %w", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		tick, ok := parseTickerMessage(data)
		if !ok {
			continue
		}
		onTick(tick)
	}
}

func FetchKlines(client *http.Client, symbol, interval string, limit int) ([]binance.Candle, error) {
	if client == nil {
		client = http.DefaultClient
	}
	interval, err := binance.NormalizeInterval(interval)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}
	if limit > 1500 {
		limit = 1500
	}
	var body any
	if err := getJSON(client, klinesPath, url.Values{
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(limit)},
	}, &body); err != nil {
		return nil, err
	}
	candles := parseKlineRows(unwrapData(body))
	if len(candles) == 0 {
		return nil, fmt.Errorf("alpha klines %s: no candles", symbol)
	}
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	return candles, nil
}

func fetchResolveDocs(client *http.Client) []any {
	now := time.Now()
	resolveCache.Lock()
	if now.Before(resolveCache.expiresAt) && len(resolveCache.docs) > 0 {
		docs := append([]any(nil), resolveCache.docs...)
		resolveCache.Unlock()
		return docs
	}
	resolveCache.Unlock()

	var docs []any
	for _, path := range []string{tokenListPath, exchangePath} {
		var body any
		if err := getJSON(client, path, nil, &body); err == nil {
			docs = append(docs, unwrapData(body))
		}
	}
	if len(docs) > 0 {
		resolveCache.Lock()
		resolveCache.docs = append([]any(nil), docs...)
		resolveCache.expiresAt = now.Add(6 * time.Hour)
		resolveCache.Unlock()
	}
	return docs
}

func resolveAlphaSymbol(item config.AlphaItem, base, quoteAsset string, docs []any) string {
	if alphaSymbolPattern.MatchString(item.Symbol) {
		return strings.ToUpper(item.Symbol)
	}
	quoteAsset = strings.ToUpper(strings.TrimSpace(quoteAsset))
	needles := []string{
		strings.ToUpper(item.Symbol),
		strings.ToUpper(strings.TrimSuffix(item.Symbol, quoteAsset)),
		strings.ToUpper(item.ID),
		strings.ToUpper(item.Name),
		base,
	}
	fuzzyNeedles := []string{
		strings.ToUpper(item.Symbol),
		strings.ToUpper(strings.TrimSuffix(item.Symbol, quoteAsset)),
		strings.ToUpper(item.ID),
		base,
	}
	for _, doc := range docs {
		if sym := findAlphaSymbol(doc, needles, quoteAsset, true); sym != "" {
			return sym
		}
	}
	for _, doc := range docs {
		if sym := findAlphaSymbol(doc, fuzzyNeedles, quoteAsset, false); sym != "" {
			return sym
		}
	}
	return item.Symbol
}

func findAlphaSymbol(v any, needles []string, quoteAsset string, exact bool) string {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			if sym := findAlphaSymbol(item, needles, quoteAsset, exact); sym != "" {
				return sym
			}
		}
	case map[string]any:
		if !objectMatches(x, needles, quoteAsset, exact) {
			for _, child := range x {
				if sym := findAlphaSymbol(child, needles, quoteAsset, exact); sym != "" {
					return sym
				}
			}
			return ""
		}
		if sym := objectAlphaSymbol(x, quoteAsset); sym != "" {
			return sym
		}
		for _, child := range x {
			if sym := findAlphaSymbol(child, needles, quoteAsset, exact); sym != "" {
				return sym
			}
		}
	}
	return ""
}

func objectMatches(m map[string]any, needles []string, quoteAsset string, exact bool) bool {
	for _, v := range m {
		s, ok := v.(string)
		if !ok {
			continue
		}
		up := strings.ToUpper(s)
		for _, needle := range needles {
			if needle == "" {
				continue
			}
			if exact {
				if matchKey(up, quoteAsset) == matchKey(needle, quoteAsset) {
					return true
				}
				continue
			}
			if up == needle || (len(matchKey(needle, quoteAsset)) >= 4 && strings.Contains(up, needle)) {
				return true
			}
		}
	}
	return false
}

func matchKey(s, quoteAsset string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if quoteAsset != "" {
		s = strings.TrimSuffix(s, quoteAsset)
	}
	return s
}

func objectAlphaSymbol(m map[string]any, quoteAsset string) string {
	for _, v := range m {
		if s, ok := v.(string); ok {
			up := strings.ToUpper(strings.TrimSpace(s))
			if alphaSymbolPattern.MatchString(up) {
				return up
			}
		}
	}
	for _, key := range []string{"tokenId", "alphaId", "id"} {
		s := strings.ToUpper(stringify(m[key]))
		if s == "" {
			continue
		}
		if alphaSymbolPattern.MatchString(s + quoteAsset) {
			return s + quoteAsset
		}
		if alphaSymbolPattern.MatchString(s) {
			return s
		}
		s = strings.TrimPrefix(s, "ALPHA_")
		if allDigits(s) {
			return "ALPHA_" + s + quoteAsset
		}
	}
	return ""
}

func parseTickerMessage(data []byte) (Ticker, bool) {
	var wrap combinedMessage
	if err := json.Unmarshal(data, &wrap); err == nil && len(wrap.Data) > 0 {
		var ev tickerEvent
		if json.Unmarshal(wrap.Data, &ev) == nil && ev.Symbol != "" {
			return normalizeTicker(ev)
		}
	}
	var ev tickerEvent
	if err := json.Unmarshal(data, &ev); err != nil || ev.Symbol == "" {
		return Ticker{}, false
	}
	return normalizeTicker(ev)
}

func normalizeTicker(ev tickerEvent) (Ticker, bool) {
	price, err := strconv.ParseFloat(ev.Close, 64)
	if err != nil || price <= 0 {
		return Ticker{}, false
	}
	change24h, _ := strconv.ParseFloat(ev.PriceChangePct, 64)
	volume, _ := strconv.ParseFloat(ev.Volume, 64)
	updatedAt := time.Now().UTC()
	if ev.EventTimeMs > 0 {
		updatedAt = time.UnixMilli(ev.EventTimeMs).UTC()
	}
	return Ticker{
		Symbol:       strings.ToUpper(ev.Symbol),
		Price:        price,
		Change24hPct: change24h,
		Volume:       volume,
		UpdatedAt:    updatedAt,
	}, true
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
	req.Header.Set("User-Agent", "binance-alpha/1.1.0 (MarketPulse)")
	req.Header.Set("Accept", "application/json,text/plain,*/*")
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
		return fmt.Errorf("alpha http %d: %s", resp.StatusCode, truncate(string(data), 180))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return err
	}
	return nil
}

func unwrapData(v any) any {
	for {
		m, ok := v.(map[string]any)
		if !ok {
			return v
		}
		next, ok := m["data"]
		if !ok {
			return v
		}
		v = next
	}
}

func firstFloat(v any, keys ...string) float64 {
	m, ok := v.(map[string]any)
	if !ok {
		return 0
	}
	for _, key := range keys {
		if n := parseAnyFloat(m[key]); n != 0 {
			return n
		}
	}
	return 0
}

func parseKlineRows(v any) []binance.Candle {
	rows, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]binance.Candle, 0, len(rows))
	for _, row := range rows {
		switch x := row.(type) {
		case []any:
			if len(x) < 6 {
				continue
			}
			t := int64(parseAnyFloat(x[0]))
			if t > 1e12 {
				t = t / 1000
			}
			open := parseAnyFloat(x[1])
			high := parseAnyFloat(x[2])
			low := parseAnyFloat(x[3])
			closep := parseAnyFloat(x[4])
			if t <= 0 || open <= 0 || high <= 0 || low <= 0 || closep <= 0 {
				continue
			}
			out = append(out, binance.Candle{
				Time:   t,
				Open:   open,
				High:   high,
				Low:    low,
				Close:  closep,
				Volume: parseAnyFloat(x[5]),
			})
		case map[string]any:
			t := int64(firstFloat(x, "openTime", "time", "t"))
			if t > 1e12 {
				t = t / 1000
			}
			open := firstFloat(x, "open", "o")
			high := firstFloat(x, "high", "h")
			low := firstFloat(x, "low", "l")
			closep := firstFloat(x, "close", "c")
			if t <= 0 || open <= 0 || high <= 0 || low <= 0 || closep <= 0 {
				continue
			}
			out = append(out, binance.Candle{
				Time:   t,
				Open:   open,
				High:   high,
				Low:    low,
				Close:  closep,
				Volume: firstFloat(x, "volume", "v"),
			})
		}
	}
	return out
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	default:
		return ""
	}
}

func parseAnyFloat(v any) float64 {
	switch x := v.(type) {
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	case float64:
		return x
	case json.Number:
		n, _ := x.Float64()
		return n
	default:
		return 0
	}
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
