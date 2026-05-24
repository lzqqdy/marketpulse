package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/marketdata"
	"github.com/lzqqdy/marketpulse/internal/marketdata/binance"
)

func (h *Handler) Klines(c *gin.Context) {
	symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_SYMBOL", "message": "symbol required"}})
		return
	}

	interval := c.DefaultQuery("interval", "1h")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(binance.DefaultKlineLimit)))
	resp, err := h.MarketData.Klines(symbol, interval, limit)
	if err != nil {
		if errors.Is(err, marketdata.ErrInvalidSymbol) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_SYMBOL", "message": "symbol not in watchlist"}})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) IndexKlines(c *gin.Context) {
	id := strings.ToLower(strings.TrimSpace(c.Query("id")))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_INDEX", "message": "id required"}})
		return
	}

	interval := c.DefaultQuery("interval", "1d")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(binance.DefaultKlineLimit)))

	resp, err := h.MarketData.IndexKlines(id, interval, limit)
	if err != nil {
		if errors.Is(err, marketdata.ErrInvalidIndex) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_INDEX", "message": "index not supported"}})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}
