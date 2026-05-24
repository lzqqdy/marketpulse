package api

import (
	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
)

// StreamWS pushes live quotes via WebSocket.
func (h *Handler) StreamWS(c *gin.Context) {
	conn, err := marketdata.ServeWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.MarketData.ServeStreamWS(conn, c.Query("channels"))
}

// KlineWS streams kline snapshot + live updates (no client polling).
func (h *Handler) KlineWS(c *gin.Context) {
	conn, err := marketdata.ServeWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", "1h")
	h.MarketData.ServeKlineWS(conn, symbol, interval)
}
