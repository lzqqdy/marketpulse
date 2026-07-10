package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/marketdata/store"
)

func normalizeMarketParam(raw string) (string, bool) {
	market := strings.ToLower(strings.TrimSpace(raw))
	if market == "" {
		market = "cn"
	}
	if market != "cn" {
		return "", false
	}
	return market, true
}

func (h *Handler) Internals(c *gin.Context) {
	if _, ok := normalizeMarketParam(c.Query("market")); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_MARKET", "message": "only market=cn is supported"}})
		return
	}
	c.JSON(http.StatusOK, h.MarketData.Internals())
}

func (h *Handler) Breadth(c *gin.Context) {
	if _, ok := normalizeMarketParam(c.Query("market")); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_MARKET", "message": "only market=cn is supported"}})
		return
	}
	cn := h.MarketData.Internals().CN
	if cn.UpdatedAt.IsZero() {
		c.JSON(http.StatusOK, store.MarketBreadth{Source: "eastmoney"})
		return
	}
	c.JSON(http.StatusOK, cn.Breadth)
}

func (h *Handler) Sectors(c *gin.Context) {
	if _, ok := normalizeMarketParam(c.Query("market")); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_MARKET", "message": "only market=cn is supported"}})
		return
	}
	sectorType := strings.ToLower(strings.TrimSpace(c.Query("type")))
	cn := h.MarketData.Internals().CN
	switch sectorType {
	case "industry":
		c.JSON(http.StatusOK, gin.H{"type": "industry", "sectors": cn.Industry, "updatedAt": cn.UpdatedAt})
	case "concept":
		c.JSON(http.StatusOK, gin.H{"type": "concept", "sectors": cn.Concept, "updatedAt": cn.UpdatedAt})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_PARAMS", "message": "type must be industry or concept"}})
	}
}

func (h *Handler) MarketWind(c *gin.Context) {
	if _, ok := normalizeMarketParam(c.Query("market")); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_MARKET", "message": "only market=cn is supported"}})
		return
	}
	cn := h.MarketData.Internals().CN
	if cn.UpdatedAt.IsZero() {
		c.JSON(http.StatusOK, store.MarketWind{Summary: "暂无 A 股市场风向数据"})
		return
	}
	c.JSON(http.StatusOK, cn.Wind)
}
