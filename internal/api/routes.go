package api

import "github.com/gin-gonic/gin"

// Register mounts all API routes on the Gin engine.
func Register(r *gin.Engine, h *Handler) {
	RegisterHealthRoutes(r, h)
	RegisterMarketRoutes(r, h)
	RegisterLegacyMarketRoutes(r, h)
	RegisterUsersRoutes(r, h)
	RegisterAlertsRoutes(r, h)
	RegisterPortfolioRoutes(r, h)
}

// RegisterPortfolioRoutes mounts /api/v1/portfolio endpoints.
func RegisterPortfolioRoutes(r *gin.Engine, h *Handler) {
	g := r.Group("/api/v1/portfolio")
	g.GET("/holdings", h.GetPortfolioHoldings)
	g.PUT("/holdings", h.PutPortfolioHoldings)
	g.PUT("/settings", h.PutPortfolioSettings)
	g.GET("/overview", h.GetPortfolioOverview)
	g.GET("/snapshots", h.ListPortfolioSnapshots)
	g.GET("/eligible-symbols", h.GetPortfolioEligibleSymbols)
	g.GET("/reports/series", h.GetPortfolioReportSeries)
	g.GET("/reports/allocation", h.GetPortfolioReportAllocation)
}

// RegisterUsersRoutes mounts /api/v1/users endpoints.
func RegisterUsersRoutes(r *gin.Engine, h *Handler) {
	g := r.Group("/api/v1/users")
	g.POST("/login", h.Login)
	g.POST("/logout", h.Logout)
	g.GET("/me", h.Me)
	g.PUT("/me", h.UpdateMe)
	g.PUT("/me/password", h.ChangePassword)
	g.POST("/me/avatar", h.UploadAvatar)
}

// RegisterAlertsRoutes mounts /api/v1/alerts endpoints and WS stream.
func RegisterAlertsRoutes(r *gin.Engine, h *Handler) {
	g := r.Group("/api/v1/alerts")
	g.GET("/rules", h.ListAlertRules)
	g.POST("/rules", h.CreateAlertRule)
	g.PATCH("/rules/:id", h.UpdateAlertRule)
	g.DELETE("/rules/:id", h.DeleteAlertRule)
	g.GET("/deliveries", h.ListAlertDeliveries)
	g.POST("/inbox/ack", h.AckAlertInbox)

	r.GET("/ws/v1/alerts/stream", h.AlertsStreamWS)
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
	market.GET("/center", h.MarketCenter)
	market.GET("/center/heatmap", h.MarketCenterHeatmap)
	market.GET("/expressnews", h.ExpressNews)

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
