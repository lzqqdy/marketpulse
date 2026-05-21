package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/config"
	"github.com/lzqqdy/marketpulse/internal/hub"
	"github.com/lzqqdy/marketpulse/internal/ingest"
	"github.com/lzqqdy/marketpulse/internal/store"
)

// Handler serves HTTP JSON endpoints.
type Handler struct {
	Config    *config.Config
	Store     *store.MarketStore
	StreamHub *hub.StreamHub
	KlineHub  *hub.KlineHub
	Ingest    *ingest.Service
	StartedAt time.Time
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

// Register mounts routes on the Gin engine.
func Register(r *gin.Engine, h *Handler) {
	r.GET("/healthz", h.Healthz)
	r.GET("/api/v1/snapshot", h.Snapshot)
	r.GET("/api/v1/providers/status", h.ProviderStatus)
	r.GET("/api/v1/klines", h.Klines)
	r.GET("/api/v1/index-klines", h.IndexKlines)
	r.GET("/ws/v1/kline", h.KlineWS)
	r.GET("/ws/v1/stream", h.StreamWS)
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:       "ok",
		UptimeSec:    int64(time.Since(h.StartedAt).Seconds()),
		SymbolCount:  len(h.Config.Symbols),
		StoreVersion: h.Store.Version(),
		AppMode:      h.Config.App.Mode,
		Ingest:       h.ingestStatus(),
	})
}

func (h *Handler) Snapshot(c *gin.Context) {
	c.JSON(http.StatusOK, h.Store.GetSnapshot())
}

func (h *Handler) ProviderStatus(c *gin.Context) {
	if h.Ingest == nil {
		c.JSON(http.StatusOK, ingest.ProviderStatusResponse{
			Overall: ingest.ProviderOverall{
				Status:    ingest.ProviderDisabled,
				UpdatedAt: time.Now().Unix(),
			},
			Providers: []ingest.ProviderHealth{},
		})
		return
	}
	c.JSON(http.StatusOK, h.Ingest.ProviderStatus())
}

func (h *Handler) ingestStatus() map[string]string {
	if h.Ingest == nil {
		return map[string]string{"binance_ws": "disabled"}
	}
	out := map[string]string{
		"binance_ws":     h.Ingest.BinanceStatus(),
		"alpha_ws":       h.Ingest.AlphaStatus(),
		"last_quote_ms":  formatLastQuote(h.Ingest.LastQuoteMs()),
		"last_alpha_ms":  formatLastQuote(h.Ingest.LastAlphaMs()),
		"stream_clients": streamClientCount(h.StreamHub),
	}
	for k, v := range h.Ingest.IngestStatus() {
		out[k] = v
	}
	return out
}

func streamClientCount(h *hub.StreamHub) string {
	if h == nil {
		return "0"
	}
	return strconv.Itoa(h.ClientCount())
}

func formatLastQuote(ms int64) string {
	if ms == 0 {
		return "never"
	}
	return time.UnixMilli(ms).Format(time.RFC3339)
}
