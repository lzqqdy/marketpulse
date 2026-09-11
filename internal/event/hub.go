package event

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub broadcasts public event WebSocket messages to all connected clients.
type Hub struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]*eventClient
}

type eventClient struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

// NewHub creates an empty broadcast hub.
func NewHub() *Hub {
	return &Hub{conns: make(map[*websocket.Conn]*eventClient)}
}

// Register adds a connection.
func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.conns[conn]; exists {
		return
	}
	c := &eventClient{conn: conn, send: make(chan []byte, 64), done: make(chan struct{})}
	h.conns[conn] = c
	go h.writeLoop(c)
}

// Unregister removes a connection.
func (h *Hub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c, ok := h.conns[conn]; ok {
		delete(h.conns, conn)
		close(c.done)
		_ = conn.Close()
	}
}

func (h *Hub) writeLoop(c *eventClient) {
	defer h.Unregister(c.conn)
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}

// Send queues both application heartbeats and events through the same writer.
func (h *Hub) Send(conn *websocket.Conn, payload []byte) {
	h.mu.RLock()
	c := h.conns[conn]
	h.mu.RUnlock()
	if c == nil {
		return
	}
	select {
	case <-c.done:
	case c.send <- append([]byte(nil), payload...):
	default:
		h.Unregister(conn)
	}
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
		h.Send(c, payload)
	}
}

// Len returns active connection count.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
