package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/alpha"
	"github.com/lzqqdy/marketpulse/internal/marketdata/ingest/bitget"
	"github.com/lzqqdy/marketpulse/internal/marketdata/requestclient"
)

// KlineHub streams kline snapshots + live updates to browser clients.
type KlineHub struct {
	cfg          *config.Config
	mu           sync.Mutex
	subs         map[string]*klineSub
	fetchSpot    func(context.Context, string, string, int) ([]binance.Candle, error)
	streamSpot   func(context.Context, string, string, func(binance.Candle)) error
	retryDelay   time.Duration
	writeTimeout time.Duration
}

type klineSub struct {
	symbol    string
	interval  string
	candles   []binance.Candle
	clients   map[*websocket.Conn]*klineClient
	cancel    context.CancelFunc
	ctx       context.Context
	source    string
	lastError []byte
}

type klineClient struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

// KlineSnapshotMsg is the initial history payload.
type KlineSnapshotMsg struct {
	Type     string           `json:"type"`
	Symbol   string           `json:"symbol"`
	Interval string           `json:"interval"`
	Candles  []binance.Candle `json:"candles"`
	Source   string           `json:"source"`
}

// KlineUpdateMsg is a live candle patch.
type KlineUpdateMsg struct {
	Type     string         `json:"type"`
	Symbol   string         `json:"symbol"`
	Interval string         `json:"interval"`
	Candle   binance.Candle `json:"candle"`
}

// NewKlineHub creates a kline subscription manager.
func NewKlineHub(cfg *config.Config) *KlineHub {
	return &KlineHub{
		cfg:          cfg,
		subs:         make(map[string]*klineSub),
		fetchSpot:    binance.FetchKlinesContext,
		streamSpot:   binance.StreamKline,
		retryDelay:   3 * time.Second,
		writeTimeout: 5 * time.Second,
	}
}

func subKey(symbol, interval string) string {
	return symbol + "|" + interval
}

func (h *KlineHub) symbolAllowed(symbol string) bool {
	for _, s := range h.cfg.Symbols {
		if s == symbol {
			return true
		}
	}
	return h.cfg.Alpha.Enabled && h.cfg.IsAlphaBaseSymbol(symbol)
}

func (h *KlineHub) isAlphaSymbol(symbol string) bool {
	return h.cfg.Alpha.Enabled && h.cfg.IsAlphaBaseSymbol(symbol)
}

func (h *KlineHub) alphaPair(client *http.Client, symbol string) (string, bool) {
	if h.cfg.Alpha.Provider == "bitget" {
		item, _, ok := h.cfg.AlphaByBaseSymbol(symbol)
		if !ok {
			return "", false
		}
		return strings.ToUpper(strings.TrimSpace(item.Symbol)), true
	}
	resolved := alpha.ResolveItems(client, h.cfg.Alpha.Indices, h.cfg.Alpha.Stocks, h.cfg.Alpha.QuoteAsset)
	for _, item := range resolved {
		if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
			return item.AlphaSymbol, true
		}
	}
	return "", false
}

// ServeWS handles GET /ws/v1/kline?symbol=BTC&interval=1h
func (h *KlineHub) ServeWS(conn *websocket.Conn, symbol, interval string) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	interval, err := binance.NormalizeInterval(interval)
	if err != nil {
		_ = conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
		_ = conn.WriteJSON(map[string]any{
			"type":    "error",
			"code":    "INVALID_PARAMS",
			"message": err.Error(),
		})
		_ = conn.Close()
		return
	}
	if !h.symbolAllowed(symbol) {
		_ = conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
		_ = conn.WriteJSON(map[string]any{
			"type":    "error",
			"code":    "INVALID_SYMBOL",
			"message": fmt.Sprintf("symbol %s not in watchlist", symbol),
		})
		_ = conn.Close()
		return
	}

	key := subKey(symbol, interval)
	h.mu.Lock()
	sub, ok := h.subs[key]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		sub = &klineSub{
			symbol:   symbol,
			interval: interval,
			clients:  make(map[*websocket.Conn]*klineClient),
			ctx:      ctx,
			cancel:   cancel,
		}
		h.subs[key] = sub
	}
	client := &klineClient{conn: conn, send: make(chan []byte, 64), done: make(chan struct{})}
	sub.clients[conn] = client
	if len(sub.candles) > 0 {
		payload, _ := json.Marshal(KlineSnapshotMsg{
			Type:     "kline_snapshot",
			Symbol:   symbol,
			Interval: interval,
			Candles:  append([]binance.Candle(nil), sub.candles...),
			Source:   sub.source,
		})
		client.send <- payload
	}
	if len(sub.lastError) > 0 {
		client.send <- sub.lastError
	}
	h.mu.Unlock()
	go h.writeClient(key, client)
	if !ok {
		go h.runSubscription(key, sub)
	}

	defer h.removeClient(key, conn)

	// read loop: handle ping / detect disconnect
	conn.SetReadDeadline(time.Time{})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *KlineHub) runSubscription(key string, sub *klineSub) {
	ctx := sub.ctx
	defer sub.cancel()
	for ctx.Err() == nil {
		var err error
		if h.isAlphaSymbol(sub.symbol) {
			err = h.runAlphaSubscription(ctx, key, sub)
		} else {
			var candles []binance.Candle
			candles, err = h.fetchSpot(ctx, sub.symbol, sub.interval, binance.DefaultKlineLimit)
			if err == nil && len(candles) == 0 {
				err = fmt.Errorf("kline history: no candles")
			}
			if err == nil {
				h.publishSnapshot(key, sub, candles, "binance")
				err = h.streamSpot(ctx, sub.symbol, sub.interval, func(c binance.Candle) { h.applyCandle(key, sub, c) })
			}
		}
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("upstream kline stream closed")
		}
		slog.Warn("kline subscription retry", "symbol", sub.symbol, "interval", sub.interval, "err", err)
		h.broadcastError(key, sub, "UPSTREAM_ERROR", err.Error())
		timer := time.NewTimer(h.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (h *KlineHub) runAlphaSubscription(ctx context.Context, key string, sub *klineSub) error {
	client := requestclient.WithContext(ctx, nil)
	alphaPair, ok := h.alphaPair(client, sub.symbol)
	if !ok {
		msg := fmt.Sprintf("alpha symbol %s not resolved", sub.symbol)
		slog.Warn("alpha kline unresolved", "symbol", sub.symbol)
		return fmt.Errorf("%s", msg)
	}
	var candles []binance.Candle
	var err error
	source := "binance-alpha"
	usingFallback := false
	if h.cfg.Alpha.Provider == "bitget" {
		source = "bitget"
		candles, err = bitget.FetchKlines(client, alphaPair, h.cfg.Alpha.ProductType, sub.interval, binance.DefaultKlineLimit)
		if err != nil && ctx.Err() == nil {
			if fallbackPair, ok := h.binanceAlphaPair(client, sub.symbol); ok {
				if fallbackCandles, fallbackErr := alpha.FetchKlines(client, fallbackPair, sub.interval, binance.DefaultKlineLimit); fallbackErr == nil {
					alphaPair = fallbackPair
					candles = fallbackCandles
					err = nil
					source = "binance-alpha"
					usingFallback = true
				}
			}
		}
	} else {
		candles, err = alpha.FetchKlines(client, alphaPair, sub.interval, binance.DefaultKlineLimit)
	}
	if err == nil && len(candles) == 0 {
		err = fmt.Errorf("reference kline history: no candles")
	}
	if err != nil {
		slog.Error("alpha kline history", "symbol", sub.symbol, "alpha_symbol", alphaPair, "interval", sub.interval, "err", err)
		return err
	}

	h.publishSnapshot(key, sub, candles, source)

	if h.cfg.Alpha.Provider == "bitget" && !usingFallback {
		return h.runBitgetAlphaKlineLive(ctx, key, sub, alphaPair)
	}
	return h.runAlphaKlinePoll(ctx, key, sub, alphaPair, true)
}

func (h *KlineHub) alphaPollInterval() time.Duration {
	if h.cfg.Alpha.PollInterval > 0 {
		return h.cfg.Alpha.PollInterval
	}
	return 30 * time.Second
}

func (h *KlineHub) runBitgetAlphaKlineLive(ctx context.Context, key string, sub *klineSub, alphaPair string) error {
	wsCtx, wsCancel := context.WithCancel(ctx)
	defer wsCancel()
	wsDone := make(chan error, 1)
	go func() {
		wsDone <- bitget.StreamKline(wsCtx, h.cfg.Alpha.ProductType, alphaPair, sub.interval, func(c binance.Candle) {
			h.applyCandle(key, sub, c)
		})
	}()

	select {
	case <-ctx.Done():
		wsCancel()
		<-wsDone
		return ctx.Err()
	case err := <-wsDone:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			slog.Info("bitget alpha kline stream ended", "symbol", sub.symbol, "alpha_symbol", alphaPair, "interval", sub.interval, "err", err)
		}
	}
	return h.runAlphaKlinePoll(ctx, key, sub, alphaPair, false)
}

func (h *KlineHub) runAlphaKlinePoll(ctx context.Context, key string, sub *klineSub, alphaPair string, binanceOnly bool) error {
	client := requestclient.WithContext(ctx, nil)
	ticker := time.NewTicker(h.alphaPollInterval())
	defer ticker.Stop()
	useBinance := binanceOnly || h.cfg.Alpha.Provider != "bitget"
	pair := alphaPair

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !useBinance {
			fresh, err := bitget.FetchKlines(client, pair, h.cfg.Alpha.ProductType, sub.interval, 2)
			if err == nil && len(fresh) > 0 {
				for _, c := range fresh {
					h.applyCandle(key, sub, c)
				}
			} else {
				if fallbackPair, ok := h.binanceAlphaPair(client, sub.symbol); ok {
					// Switching instruments replaces history instead of mixing providers.
					candles, fallbackErr := alpha.FetchKlines(client, fallbackPair, sub.interval, binance.DefaultKlineLimit)
					if fallbackErr != nil {
						return fallbackErr
					}
					h.publishSnapshot(key, sub, candles, "binance-alpha")
					pair = fallbackPair
					useBinance = true
				} else if err != nil {
					return err
				} else {
					return fmt.Errorf("bitget kline poll: no candles")
				}
			}
		}
		if useBinance {
			fresh, err := alpha.FetchKlines(client, pair, sub.interval, 2)
			if err != nil {
				return err
			} else if len(fresh) > 0 {
				for _, c := range fresh {
					h.applyCandle(key, sub, c)
				}
			} else {
				return fmt.Errorf("alpha kline poll: no candles")
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (h *KlineHub) binanceAlphaPair(client *http.Client, symbol string) (string, bool) {
	resolved := alpha.ResolveItems(client, h.cfg.Alpha.Indices, h.cfg.Alpha.Stocks, h.cfg.Alpha.QuoteAsset)
	for _, item := range resolved {
		if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
			return item.AlphaSymbol, true
		}
	}
	return "", false
}

func (h *KlineHub) applyCandle(key string, sub *klineSub, c binance.Candle) {
	h.mu.Lock()
	if h.subs[key] != sub || sub.ctx.Err() != nil {
		h.mu.Unlock()
		return
	}
	sub.candles = mergeCandle(sub.candles, c)
	if len(sub.candles) > binance.DefaultKlineLimit {
		sub.candles = sub.candles[len(sub.candles)-binance.DefaultKlineLimit:]
	}
	msg, _ := json.Marshal(KlineUpdateMsg{
		Type:     "kline_update",
		Symbol:   sub.symbol,
		Interval: sub.interval,
		Candle:   c,
	})
	failed := h.queueLocked(sub, msg)
	h.mu.Unlock()
	for _, conn := range failed {
		h.removeClient(key, conn)
	}
}

func (h *KlineHub) publishSnapshot(key string, sub *klineSub, candles []binance.Candle, source string) {
	h.mu.Lock()
	if h.subs[key] != sub || sub.ctx.Err() != nil {
		h.mu.Unlock()
		return
	}
	sub.candles = append([]binance.Candle(nil), candles...)
	sub.source = source
	sub.lastError = nil
	msg, _ := json.Marshal(KlineSnapshotMsg{Type: "kline_snapshot", Symbol: sub.symbol, Interval: sub.interval, Candles: sub.candles, Source: source})
	failed := h.queueLocked(sub, msg)
	h.mu.Unlock()
	for _, conn := range failed {
		h.removeClient(key, conn)
	}
}

func mergeCandle(candles []binance.Candle, c binance.Candle) []binance.Candle {
	if len(candles) == 0 {
		return []binance.Candle{c}
	}
	last := candles[len(candles)-1]
	if last.Time == c.Time {
		candles[len(candles)-1] = c
		return candles
	}
	if c.Time > last.Time {
		return append(candles, c)
	}
	// out-of-order: replace if exists
	for i := range candles {
		if candles[i].Time == c.Time {
			candles[i] = c
			return candles
		}
	}
	return candles
}

func (h *KlineHub) queueLocked(sub *klineSub, msg []byte) []*websocket.Conn {
	var failed []*websocket.Conn
	for conn, client := range sub.clients {
		select {
		case client.send <- msg:
		default:
			failed = append(failed, conn)
		}
	}
	return failed
}

func (h *KlineHub) broadcastError(key string, sub *klineSub, code, msg string) {
	payload, _ := json.Marshal(map[string]any{
		"type":    "error",
		"code":    code,
		"message": msg,
	})
	h.mu.Lock()
	if h.subs[key] != sub {
		h.mu.Unlock()
		return
	}
	sub.lastError = payload
	failed := h.queueLocked(sub, payload)
	h.mu.Unlock()
	for _, conn := range failed {
		h.removeClient(key, conn)
	}
}

func (h *KlineHub) writeClient(key string, client *klineClient) {
	defer h.removeClient(key, client.conn)
	for {
		select {
		case <-client.done:
			return
		case msg := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
			if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}

func (h *KlineHub) removeClient(key string, conn *websocket.Conn) {
	h.mu.Lock()
	sub, ok := h.subs[key]
	if !ok {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	client, ok := sub.clients[conn]
	if !ok {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	delete(sub.clients, conn)
	close(client.done)
	if len(sub.clients) == 0 {
		if sub.cancel != nil {
			sub.cancel()
		}
		delete(h.subs, key)
	}
	h.mu.Unlock()
	_ = conn.Close()
}
