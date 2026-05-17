package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/binance"
	"github.com/lzqqdy/marketpulse/internal/config"
)

// KlineHub streams kline snapshots + live updates to browser clients.
type KlineHub struct {
	cfg *config.Config
	mu  sync.Mutex
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
	return false
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
				Source:   "binance",
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

	candles, err := binance.FetchKlines(sub.symbol, sub.interval, 300)
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
