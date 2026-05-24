package api

import "github.com/gin-gonic/gin"

// Register mounts all API routes on the Gin engine.
func Register(r *gin.Engine, h *Handler) {
	RegisterHealthRoutes(r, h)
	RegisterMarketRoutes(r, h)
	RegisterLegacyMarketRoutes(r, h)
}

// RegisterHealthRoutes mounts process-level health endpoints.
func RegisterHealthRoutes(r *gin.Engine, h *Handler) {
	r.GET("/healthz", h.Healthz)
}

// RegisterMarketRoutes mounts the canonical market data API namespace.
func RegisterMarketRoutes(r *gin.Engine, h *Handler) {
	market := r.Group("/api/v1/market")
	market.GET("/snapshot", h.Snapshot)
	market.GET("/providers/status", h.ProviderStatus)
	market.GET("/klines", h.Klines)
	market.GET("/index-klines", h.IndexKlines)

	ws := r.Group("/ws/v1/market")
	ws.GET("/stream", h.StreamWS)
	ws.GET("/kline", h.KlineWS)
}

// RegisterLegacyMarketRoutes keeps pre-market-namespace clients working.
func RegisterLegacyMarketRoutes(r *gin.Engine, h *Handler) {
	r.GET("/api/v1/snapshot", h.Snapshot)
	r.GET("/api/v1/providers/status", h.ProviderStatus)
	r.GET("/api/v1/klines", h.Klines)
	r.GET("/api/v1/index-klines", h.IndexKlines)
	r.GET("/ws/v1/stream", h.StreamWS)
	r.GET("/ws/v1/kline", h.KlineWS)
}
