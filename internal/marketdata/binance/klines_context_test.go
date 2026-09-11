package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchKlinesContextStopsUpstream(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer srv.Close()
	old := restBase
	restBase = srv.URL
	defer func() { restBase = old }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := FetchKlinesContext(ctx, "BTC", "1m", 60); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("15-second client timeout overrode caller cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("upstream still running")
	}
}
