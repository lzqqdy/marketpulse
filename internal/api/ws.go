package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/hub"
)

// StreamWS pushes live quotes via WebSocket.
func (h *Handler) StreamWS(c *gin.Context) {
	if h.StreamHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "UNAVAILABLE", "message": "stream hub not configured"},
		})
		return
	}
	conn, err := hub.ServeWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.StreamHub.ServeWS(conn, c.Query("channels"))
}

// KlineWS streams kline snapshot + live updates (no client polling).
func (h *Handler) KlineWS(c *gin.Context) {
	if h.KlineHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{"code": "UNAVAILABLE", "message": "kline hub not configured"},
		})
		return
	}
	conn, err := hub.ServeWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", "1h")
	h.KlineHub.ServeWS(conn, symbol, interval)
}
