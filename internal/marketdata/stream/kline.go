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
)

// KlineHub streams kline snapshots + live updates to browser clients.
type KlineHub struct {
	cfg  *config.Config
	mu   sync.Mutex
	subs map[string]*klineSub
}

type klineSub struct {
	symbol   string
	interval string
	candles  []binance.Candle
	clients  map[*websocket.Conn]struct{}
	cancel   context.CancelFunc
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
		cfg:  cfg,
		subs: make(map[string]*klineSub),
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

func (h *KlineHub) alphaPair(symbol string) (string, bool) {
	if h.cfg.Alpha.Provider == "bitget" {
		item, _, ok := h.cfg.AlphaByBaseSymbol(symbol)
		if !ok {
			return "", false
		}
		return strings.ToUpper(strings.TrimSpace(item.Symbol)), true
	}
	resolved := alpha.ResolveItems(http.DefaultClient, h.cfg.Alpha.Indices, h.cfg.Alpha.Stocks, h.cfg.Alpha.QuoteAsset)
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
		_ = conn.WriteJSON(map[string]any{
			"type":    "error",
			"code":    "INVALID_PARAMS",
			"message": err.Error(),
		})
		_ = conn.Close()
		return
	}
	if !h.symbolAllowed(symbol) {
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
		sub = &klineSub{
			symbol:   symbol,
			interval: interval,
			clients:  make(map[*websocket.Conn]struct{}),
		}
		h.subs[key] = sub
	}
	sub.clients[conn] = struct{}{}
	needStart := sub.cancel == nil
	h.mu.Unlock()

	if needStart {
		go h.runSubscription(key, sub)
	} else {
		h.mu.Lock()
		if len(sub.candles) > 0 {
			_ = conn.WriteJSON(KlineSnapshotMsg{
				Type:     "kline_snapshot",
				Symbol:   symbol,
				Interval: interval,
				Candles:  append([]binance.Candle(nil), sub.candles...),
				Source:   h.klineSource(symbol),
			})
		}
		h.mu.Unlock()
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
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	sub.cancel = cancel
	h.mu.Unlock()

	if h.isAlphaSymbol(sub.symbol) {
		h.runAlphaSubscription(ctx, key, sub)
		return
	}

	candles, err := binance.FetchKlines(sub.symbol, sub.interval, binance.DefaultKlineLimit)
	if err != nil {
		slog.Error("kline history", "symbol", sub.symbol, "interval", sub.interval, "err", err)
		h.broadcastError(sub, "UPSTREAM_ERROR", err.Error())
		h.teardown(key)
		return
	}

	h.mu.Lock()
	sub.candles = candles
	clients := h.copyClients(sub)
	h.mu.Unlock()

	snap := KlineSnapshotMsg{
		Type:     "kline_snapshot",
		Symbol:   sub.symbol,
		Interval: sub.interval,
		Candles:  candles,
		Source:   "binance",
	}
	for c := range clients {
		if err := c.WriteJSON(snap); err != nil {
			h.removeClient(key, c)
		}
	}

	err = binance.StreamKline(ctx, sub.symbol, sub.interval, func(c binance.Candle) {
		h.applyCandle(key, sub, c)
	})
	if err != nil && ctx.Err() == nil {
		slog.Warn("kline stream ended", "symbol", sub.symbol, "interval", sub.interval, "err", err)
	}
	h.teardown(key)
}

func (h *KlineHub) runAlphaSubscription(ctx context.Context, key string, sub *klineSub) {
	alphaPair, ok := h.alphaPair(sub.symbol)
	if !ok {
		msg := fmt.Sprintf("alpha symbol %s not resolved", sub.symbol)
		slog.Warn("alpha kline unresolved", "symbol", sub.symbol)
		h.broadcastError(sub, "UPSTREAM_ERROR", msg)
		h.teardown(key)
		return
	}
	var candles []binance.Candle
	var err error
	source := "binance-alpha"
	usingFallback := false
	if h.cfg.Alpha.Provider == "bitget" {
		source = "bitget"
		candles, err = bitget.FetchKlines(http.DefaultClient, alphaPair, h.cfg.Alpha.ProductType, sub.interval, binance.DefaultKlineLimit)
		if err != nil {
			if fallbackPair, ok := h.binanceAlphaPair(sub.symbol); ok {
				if fallbackCandles, fallbackErr := alpha.FetchKlines(http.DefaultClient, fallbackPair, sub.interval, binance.DefaultKlineLimit); fallbackErr == nil {
					alphaPair = fallbackPair
					candles = fallbackCandles
					err = nil
					source = "binance-alpha"
					usingFallback = true
				}
			}
		}
	} else {
		candles, err = alpha.FetchKlines(http.DefaultClient, alphaPair, sub.interval, binance.DefaultKlineLimit)
	}
	if err != nil {
		slog.Error("alpha kline history", "symbol", sub.symbol, "alpha_symbol", alphaPair, "interval", sub.interval, "err", err)
		h.broadcastError(sub, "UPSTREAM_ERROR", err.Error())
		h.teardown(key)
		return
	}

	h.mu.Lock()
	sub.candles = candles
	clients := h.copyClients(sub)
	h.mu.Unlock()

	snap := KlineSnapshotMsg{
		Type:     "kline_snapshot",
		Symbol:   sub.symbol,
		Interval: sub.interval,
		Candles:  candles,
		Source:   source,
	}
	for c := range clients {
		if err := c.WriteJSON(snap); err != nil {
			h.removeClient(key, c)
		}
	}

	if h.cfg.Alpha.Provider == "bitget" && !usingFallback {
		h.runBitgetAlphaKlineLive(ctx, key, sub, alphaPair)
		return
	}
	h.runAlphaKlinePoll(ctx, key, sub, alphaPair, true)
}

func (h *KlineHub) alphaPollInterval() time.Duration {
	if h.cfg.Alpha.PollInterval > 0 {
		return h.cfg.Alpha.PollInterval
	}
	return 30 * time.Second
}

func (h *KlineHub) runBitgetAlphaKlineLive(ctx context.Context, key string, sub *klineSub, alphaPair string) {
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
		h.teardown(key)
		return
	case err := <-wsDone:
		if ctx.Err() != nil {
			h.teardown(key)
			return
		}
		if err != nil {
			slog.Info("bitget alpha kline stream ended", "symbol", sub.symbol, "alpha_symbol", alphaPair, "interval", sub.interval, "err", err)
		}
	}
	h.runAlphaKlinePoll(ctx, key, sub, alphaPair, false)
}

func (h *KlineHub) runAlphaKlinePoll(ctx context.Context, key string, sub *klineSub, alphaPair string, binanceOnly bool) {
	ticker := time.NewTicker(h.alphaPollInterval())
	defer ticker.Stop()
	useBinance := binanceOnly || h.cfg.Alpha.Provider != "bitget"
	pair := alphaPair

	for {
		if !useBinance {
			fresh, err := bitget.FetchKlines(http.DefaultClient, pair, h.cfg.Alpha.ProductType, sub.interval, 2)
			if err == nil && len(fresh) > 0 {
				h.applyCandle(key, sub, fresh[len(fresh)-1])
			} else {
				if fallbackPair, ok := h.binanceAlphaPair(sub.symbol); ok {
					pair = fallbackPair
					useBinance = true
				} else if err != nil {
					slog.Warn("alpha kline poll", "symbol", sub.symbol, "alpha_symbol", pair, "interval", sub.interval, "err", err)
				}
			}
		}
		if useBinance {
			fresh, err := alpha.FetchKlines(http.DefaultClient, pair, sub.interval, 2)
			if err != nil {
				slog.Warn("alpha kline poll", "symbol", sub.symbol, "alpha_symbol", pair, "interval", sub.interval, "err", err)
			} else if len(fresh) > 0 {
				h.applyCandle(key, sub, fresh[len(fresh)-1])
			}
		}

		select {
		case <-ctx.Done():
			h.teardown(key)
			return
		case <-ticker.C:
		}
	}
}

func (h *KlineHub) binanceAlphaPair(symbol string) (string, bool) {
	resolved := alpha.ResolveItems(http.DefaultClient, h.cfg.Alpha.Indices, h.cfg.Alpha.Stocks, h.cfg.Alpha.QuoteAsset)
	for _, item := range resolved {
		if item.BaseSymbol == symbol && item.AlphaSymbol != "" {
			return item.AlphaSymbol, true
		}
	}
	return "", false
}

func (h *KlineHub) klineSource(symbol string) string {
	if h.isAlphaSymbol(symbol) {
		if h.cfg.Alpha.Provider == "bitget" {
			return "bitget"
		}
		return "binance-alpha"
	}
	return "binance"
}

func (h *KlineHub) applyCandle(key string, sub *klineSub, c binance.Candle) {
	h.mu.Lock()
	sub.candles = mergeCandle(sub.candles, c)
	clients := h.copyClients(sub)
	h.mu.Unlock()

	msg := KlineUpdateMsg{
		Type:     "kline_update",
		Symbol:   sub.symbol,
		Interval: sub.interval,
		Candle:   c,
	}
	for conn := range clients {
		if err := conn.WriteJSON(msg); err != nil {
			h.removeClient(key, conn)
		}
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

func (h *KlineHub) copyClients(sub *klineSub) map[*websocket.Conn]struct{} {
	out := make(map[*websocket.Conn]struct{}, len(sub.clients))
	for c := range sub.clients {
		out[c] = struct{}{}
	}
	return out
}

func (h *KlineHub) broadcastError(sub *klineSub, code, msg string) {
	payload, _ := json.Marshal(map[string]any{
		"type":    "error",
		"code":    code,
		"message": msg,
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range sub.clients {
		_ = c.WriteMessage(websocket.TextMessage, payload)
	}
}

func (h *KlineHub) removeClient(key string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[key]
	if !ok {
		return
	}
	delete(sub.clients, conn)
	_ = conn.Close()
	if len(sub.clients) == 0 {
		if sub.cancel != nil {
			sub.cancel()
		}
		delete(h.subs, key)
	}
}

func (h *KlineHub) teardown(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub, ok := h.subs[key]
	if !ok {
		return
	}
	if len(sub.clients) == 0 {
		delete(h.subs, key)
		return
	}
	sub.cancel = nil
	// keep candles for reconnecting clients; stream will restart on next client if needed
}
