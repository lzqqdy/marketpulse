package binance

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCancelKlineStreamClosesBlockedRead(t *testing.T) {
	connected := make(chan struct{})
	stopped := make(chan struct{})
	u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		close(connected)
		_, _, _ = c.ReadMessage()
		close(stopped)
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- streamKlineURL(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), func(Candle) {}) }()
	<-connected
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream read did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("upstream connection remained open")
	}
}
