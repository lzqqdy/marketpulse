package hub

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/store"
)

// StreamHub broadcasts market snapshot deltas to browser clients.
type StreamHub struct {
	store    *store.MarketStore
	mu       sync.Mutex
	clients  map[*streamClient]struct{}
	debounce *time.Timer
}

type streamClient struct {
	conn     *websocket.Conn
	channels map[string]bool
	writeMu  sync.Mutex // gorilla/websocket 禁止并发 Write
}

func (c *streamClient) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(v)
}

// QuotesMsg is pushed on quotes channel.
type QuotesMsg struct {
	Type    string        `json:"type"`
	Version uint64        `json:"version"`
	Ts      int64         `json:"ts"`
	Data    []store.Quote `json:"data"`
}

// RatesMsg is pushed on rates channel.
type RatesMsg struct {
	Type    string      `json:"type"`
	Version uint64      `json:"version"`
	Ts      int64       `json:"ts"`
	Data    store.Rates `json:"data"`
}

// IndicesMsg is pushed on indices channel.
type IndicesMsg struct {
	Type    string             `json:"type"`
	Version uint64             `json:"version"`
	Ts      int64              `json:"ts"`
	Data    []store.IndexQuote `json:"data"`
}

// MacroMsg is pushed on macro channel.
type MacroMsg struct {
	Type    string              `json:"type"`
	Version uint64              `json:"version"`
	Ts      int64               `json:"ts"`
	Data    store.MacroSnapshot `json:"data"`
}

// SnapshotMsg is the initial full payload.
type SnapshotMsg struct {
	Type string          `json:"type"`
	Data store.Snapshot  `json:"data"`
}

// NewStreamHub creates a hub wired to store change notifications.
func NewStreamHub(st *store.MarketStore) *StreamHub {
	h := &StreamHub{
		store:   st,
		clients: make(map[*streamClient]struct{}),
	}
	st.AddListener(h.onStoreChange)
	return h
}

func (h *StreamHub) onStoreChange(_ uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.debounce != nil {
		return
	}
	h.debounce = time.AfterFunc(100*time.Millisecond, h.flush)
}

func (h *StreamHub) flush() {
	h.mu.Lock()
	if h.debounce != nil {
		h.debounce = nil
	}
	clients := make([]*streamClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	if len(clients) == 0 {
		return
	}

	snap := h.store.GetSnapshot()
	quotes := QuotesMsg{
		Type:    "quotes",
		Version: snap.Version,
		Ts:      snap.Ts,
		Data:    snap.Quotes,
	}
	rates := RatesMsg{
		Type:    "rates",
		Version: snap.Version,
		Ts:      snap.Ts,
		Data:    snap.Rates,
	}
	indices := IndicesMsg{
		Type:    "indices",
		Version: snap.Version,
		Ts:      snap.Ts,
		Data:    snap.Indices,
	}
	macro := MacroMsg{
		Type:    "macro",
		Version: snap.Version,
		Ts:      snap.Ts,
		Data:    snap.Macro,
	}

	for _, c := range clients {
		if c.channels["quotes"] && len(snap.Quotes) > 0 {
			if err := c.writeJSON(quotes); err != nil {
				h.removeClient(c)
			}
		}
		if c.channels["rates"] && snap.Rates.USDTCNY > 0 {
			if err := c.writeJSON(rates); err != nil {
				h.removeClient(c)
			}
		}
		if c.channels["indices"] && len(snap.Indices) > 0 {
			if err := c.writeJSON(indices); err != nil {
				h.removeClient(c)
			}
		}
		if c.channels["macro"] && snap.Macro.TotalMarketCapUsd > 0 {
			if err := c.writeJSON(macro); err != nil {
				h.removeClient(c)
			}
		}
	}
}

// ServeWS handles GET /ws/v1/stream?channels=quotes,rates
func (h *StreamHub) ServeWS(conn *websocket.Conn, channelsParam string) {
	channels := parseChannels(channelsParam)
	if len(channels) == 0 {
		channels["quotes"] = true
	}

	client := &streamClient{
		conn:     conn,
		channels: channels,
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	defer h.removeClient(client)

	// initial snapshot
	snap := h.store.GetSnapshot()
	if err := client.writeJSON(SnapshotMsg{Type: "snapshot", Data: snap}); err != nil {
		return
	}

	conn.SetReadDeadline(time.Time{})
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			Op string `json:"op"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Op == "ping" {
			_ = client.writeJSON(map[string]any{"type": "pong", "ts": time.Now().UnixMilli()})
		}
	}
}

func parseChannels(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func (h *StreamHub) removeClient(c *streamClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		_ = c.conn.Close()
	}
}

// ServeWSUpgrader is shared with api package.
var ServeWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// LogClientCount for debug.
func (h *StreamHub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
