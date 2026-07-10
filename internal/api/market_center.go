package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/marketcenter"
)

func (h *Handler) MarketCenter(c *gin.Context) {
	market := strings.ToLower(strings.TrimSpace(c.DefaultQuery("market", marketcenter.MarketAB)))
	resp, err := h.MarketData.MarketCenter(market)
	if err != nil {
		writeMarketCenterError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) MarketCenterHeatmap(c *gin.Context) {
	market := strings.ToLower(strings.TrimSpace(c.DefaultQuery("market", marketcenter.MarketAB)))
	sortKey := strings.TrimSpace(c.Query("sortKey"))
	resp, err := h.MarketData.MarketCenterHeatmap(market, sortKey)
	if err != nil {
		writeMarketCenterError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func writeMarketCenterError(c *gin.Context, err error) {
	if errors.Is(err, marketdata.ErrInvalidMarket) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "INVALID_MARKET", "message": "market must be ab, hk, or us"},
		})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
	})
}
