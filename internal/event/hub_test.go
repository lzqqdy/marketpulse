package event

import (
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentHeartbeatAndBroadcast(t *testing.T) {
	h := NewHub()
	ready := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.Register(c)
		ready <- c
		defer h.Unregister(c)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
			h.Send(c, []byte(`{"type":"pong"}`))
		}
	}))
	defer srv.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	serverConn := <-ready
	defer h.Unregister(serverConn)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			h.Broadcast([]byte(`{"type":"event"}`))
		}
	}()
	for i := 0; i < 20; i++ {
		if err := c.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
			t.Fatal(err)
		}
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 40; i++ {
		if _, _, err := c.ReadMessage(); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

func TestFullQueueDropsOnlySlowClient(t *testing.T) {
	h := NewHub()
	ready := make(chan *websocket.Conn, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := wsUpgrader.Upgrade(w, r, nil)
		if err == nil {
			ready <- c
		}
	}))
	defer srv.Close()
	connect := func() *websocket.Conn {
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		s := <-ready
		t.Cleanup(func() { _ = s.Close() })
		return s
	}
	// Leave writers stopped to deterministically simulate a full client queue.
	slow, fast := connect(), connect()
	s := &eventClient{conn: slow, send: make(chan []byte, 1), done: make(chan struct{})}
	f := &eventClient{conn: fast, send: make(chan []byte, 1), done: make(chan struct{})}
	s.send <- []byte("occupied")
	h.conns[slow], h.conns[fast] = s, f
	h.Broadcast([]byte("event"))
	if h.Len() != 1 {
		t.Fatalf("clients=%d", h.Len())
	}
	select {
	case <-s.done:
	default:
		t.Fatal("slow client not removed")
	}
	select {
	case <-f.send:
	default:
		t.Fatal("healthy client blocked")
	}
}
