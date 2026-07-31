package event

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub broadcasts public event WebSocket messages to all connected clients.
type Hub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

// NewHub creates an empty broadcast hub.
func NewHub() *Hub {
	return &Hub{conns: make(map[*websocket.Conn]struct{})}
}

// Register adds a connection.
func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = struct{}{}
}

// Unregister removes a connection.
func (h *Hub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, conn)
}

// Broadcast sends payload to all connections; failed writes are dropped.
func (h *Hub) Broadcast(payload []byte) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()
	for _, c := range conns {
		if err := c.WriteMessage(websocket.TextMessage, payload); err != nil {
			h.Unregister(c)
			_ = c.Close()
		}
	}
}

// Len returns active connection count.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
