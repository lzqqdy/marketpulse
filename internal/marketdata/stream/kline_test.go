package stream

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitKline(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("kline condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func klineTestServer(t *testing.T, h *KlineHub) *httptest.Server {
	t.Helper()
	u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.ServeWS(c, r.URL.Query().Get("symbol"), "1m")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dialKline(t *testing.T, srv *httptest.Server, symbol string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?symbol="+symbol, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	return c
}

func readKline(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_, msg, err := c.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(msg, &out); err != nil {
		t.Fatalf("corrupt WS message %q: %v", msg, err)
	}
	return out
}

func TestConcurrentClientsShareOneUpstream(t *testing.T) {
	h := NewKlineHub(&config.Config{Symbols: []string{"BTC"}})
	release := make(chan struct{})
	var fetches, streams atomic.Int32
	h.fetchSpot = func(ctx context.Context, _ string, _ string, _ int) ([]binance.Candle, error) {
		fetches.Add(1)
		select {
		case <-release:
			return []binance.Candle{{Time: 1, Close: 100}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	h.streamSpot = func(ctx context.Context, _ string, _ string, _ func(binance.Candle)) error {
		streams.Add(1)
		<-ctx.Done()
		return ctx.Err()
	}
	srv := klineTestServer(t, h)
	const count = 8
	clients := make([]*websocket.Conn, count)
	var wg sync.WaitGroup
	for i := range clients {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?symbol=BTC", nil)
			if err != nil {
				t.Error(err)
				return
			}
			clients[i] = c
		}(i)
	}
	wg.Wait()
	defer func() {
		for _, c := range clients {
			if c != nil {
				_ = c.Close()
			}
		}
	}()
	waitKline(t, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		s := h.subs["BTC|1m"]
		return s != nil && len(s.clients) == count
	})
	close(release)
	for _, c := range clients {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		if msg := readKline(t, c); msg["type"] != "kline_snapshot" {
			t.Fatalf("%v", msg)
		}
	}
	waitKline(t, func() bool { return streams.Load() == 1 })
	if fetches.Load() != 1 {
		t.Fatalf("duplicate upstream fetches=%d", fetches.Load())
	}
}

func TestUpstreamFailureRetriesOnExistingBrowserConnection(t *testing.T) {
	h := NewKlineHub(&config.Config{Symbols: []string{"BTC"}})
	h.retryDelay = time.Millisecond
	var fetches, streams atomic.Int32
	h.fetchSpot = func(context.Context, string, string, int) ([]binance.Candle, error) {
		n := fetches.Add(1)
		return []binance.Candle{{Time: 1, Close: float64(n)}}, nil
	}
	h.streamSpot = func(ctx context.Context, _ string, _ string, on func(binance.Candle)) error {
		if streams.Add(1) == 1 {
			return errors.New("upstream disconnected")
		}
		on(binance.Candle{Time: 1, Close: 200})
		<-ctx.Done()
		return ctx.Err()
	}
	c := dialKline(t, klineTestServer(t, h), "BTC")
	if msg := readKline(t, c); msg["type"] != "kline_snapshot" {
		t.Fatal(msg)
	}
	if msg := readKline(t, c); msg["type"] != "error" {
		t.Fatal(msg)
	}
	if msg := readKline(t, c); msg["type"] != "kline_snapshot" {
		t.Fatal(msg)
	}
	if msg := readKline(t, c); msg["type"] != "kline_update" {
		t.Fatal(msg)
	}
	if fetches.Load() != 2 || streams.Load() != 2 {
		t.Fatal("upstream was not restarted")
	}
}

func TestJoiningDuringHistoryFailureGetsCurrentError(t *testing.T) {
	h := NewKlineHub(&config.Config{Symbols: []string{"BTC"}})
	h.retryDelay = time.Hour
	h.fetchSpot = func(context.Context, string, string, int) ([]binance.Candle, error) {
		return nil, errors.New("history unavailable")
	}
	srv := klineTestServer(t, h)
	first := dialKline(t, srv, "BTC")
	if msg := readKline(t, first); msg["type"] != "error" {
		t.Fatal(msg)
	}
	second := dialKline(t, srv, "BTC")
	if msg := readKline(t, second); msg["type"] != "error" {
		t.Fatal("new client missed upstream failure", msg)
	}
}

func TestStaleSubscriptionCannotModifyReplacement(t *testing.T) {
	h := NewKlineHub(&config.Config{Symbols: []string{"BTC"}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	old := &klineSub{ctx: ctx, clients: map[*websocket.Conn]*klineClient{}}
	current := &klineSub{ctx: ctx, candles: []binance.Candle{{Time: 1, Close: 100}}, clients: map[*websocket.Conn]*klineClient{}}
	h.subs["BTC|1m"] = current
	h.applyCandle("BTC|1m", old, binance.Candle{Time: 1, Close: 999})
	h.publishSnapshot("BTC|1m", old, []binance.Candle{{Time: 1, Close: 999}}, "wrong")
	if current.candles[0].Close != 100 || len(old.candles) > 0 || current.source != "" {
		t.Fatal("old task modified replacement")
	}
}

func TestJoiningClientGetsOrderedSnapshotAndUpdates(t *testing.T) {
	h := NewKlineHub(&config.Config{Symbols: []string{"BTC"}})
	h.fetchSpot = func(context.Context, string, string, int) ([]binance.Candle, error) {
		return []binance.Candle{{Time: 1, Close: 100}}, nil
	}
	h.streamSpot = func(ctx context.Context, _ string, _ string, _ func(binance.Candle)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	srv := klineTestServer(t, h)
	first := dialKline(t, srv, "BTC")
	readKline(t, first)
	h.mu.Lock()
	sub := h.subs["BTC|1m"]
	sub.source = "binance-alpha"
	h.mu.Unlock()
	second := dialKline(t, srv, "BTC")
	waitKline(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(sub.clients) == 2 })
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); h.applyCandle("BTC|1m", sub, binance.Candle{Time: 1, Close: float64(i)}) }(i)
	}
	wg.Wait()
	msg := readKline(t, second)
	if msg["type"] != "kline_snapshot" || msg["source"] != "binance-alpha" {
		t.Fatalf("bad cached snapshot %v", msg)
	}
	for i := 0; i < 20; i++ {
		if msg := readKline(t, second); msg["type"] != "kline_update" {
			t.Fatal(msg)
		}
	}
}

func TestSlowClientQueueDoesNotBlockAnotherSubscription(t *testing.T) {
	h := NewKlineHub(&config.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := &klineClient{send: make(chan []byte, 1)}
	fast := &klineClient{send: make(chan []byte, 1)}
	slow.send <- []byte("occupied")
	// Distinct pointer keys are sufficient for queue selection; no network writes.
	slowConn, fastConn := &websocket.Conn{}, &websocket.Conn{}
	sub := &klineSub{ctx: ctx, clients: map[*websocket.Conn]*klineClient{slowConn: slow, fastConn: fast}}
	failed := h.queueLocked(sub, []byte("update"))
	if len(failed) != 1 || failed[0] != slowConn {
		t.Fatalf("failed=%v", failed)
	}
	select {
	case <-fast.send:
	default:
		t.Fatal("healthy client blocked")
	}
}
