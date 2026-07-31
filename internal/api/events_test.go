package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/api"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

func TestEventsDisabledReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Event.Enabled = false
	md := marketdata.New(cfg)
	h := &api.Handler{
		Config:     cfg,
		MarketData: md,
		StartedAt:  time.Now().UTC(),
	}
	r := gin.New()
	api.RegisterEventRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
