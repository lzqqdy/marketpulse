package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/store"
)

func TestKlines_invalidSymbol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		App:     config.AppConfig{Mode: "debug"},
		Symbols: []string{"BTC"},
	}
	h := &Handler{Config: cfg, Store: store.New("BTC")}
	r := gin.New()
	r.GET("/api/v1/klines", h.Klines)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/klines?symbol=DOGE", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
}
