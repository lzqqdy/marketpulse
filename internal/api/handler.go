package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

// Handler serves HTTP JSON endpoints.
type Handler struct {
	Config     *config.Config
	MarketData marketdata.MarketDataService
	StartedAt  time.Time
}

// HealthResponse is returned by GET /healthz.
type HealthResponse struct {
	Status       string            `json:"status"`
	UptimeSec    int64             `json:"uptimeSec"`
	SymbolCount  int               `json:"symbolCount"`
	StoreVersion uint64            `json:"storeVersion"`
	AppMode      string            `json:"appMode"`
	Ingest       map[string]string `json:"ingest"`
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:       "ok",
		UptimeSec:    int64(time.Since(h.StartedAt).Seconds()),
		SymbolCount:  h.MarketData.SymbolCount(),
		StoreVersion: h.MarketData.Version(),
		AppMode:      h.Config.App.Mode,
		Ingest:       h.MarketData.IngestStatus(),
	})
}

func (h *Handler) Snapshot(c *gin.Context) {
	c.JSON(http.StatusOK, h.MarketData.Snapshot())
}

func (h *Handler) ProviderStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.MarketData.ProviderStatus())
}
