package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/binance"
	"github.com/lzqqdy/marketpulse/internal/ingest/equity"
)

// KlineResponse is returned by GET /api/v1/klines.
type KlineResponse struct {
	Symbol   string           `json:"symbol"`
	Pair     string           `json:"pair"`
	Interval string           `json:"interval"`
	Candles  []binance.Candle `json:"candles"`
	Source   string           `json:"source"`
}

func (h *Handler) Klines(c *gin.Context) {
	symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_SYMBOL", "message": "symbol required"}})
		return
	}
	if !h.symbolAllowed(symbol) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_SYMBOL", "message": "symbol not in watchlist"}})
		return
	}

	interval := c.DefaultQuery("interval", "1h")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(binance.DefaultKlineLimit)))
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}

	candles, err := binance.FetchKlines(symbol, interval, limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, KlineResponse{
		Symbol:   symbol,
		Pair:     binance.SymbolUSDT(symbol),
		Interval: interval,
		Candles:  candles,
		Source:   "binance",
	})
}

func (h *Handler) IndexKlines(c *gin.Context) {
	id := strings.ToLower(strings.TrimSpace(c.Query("id")))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_INDEX", "message": "id required"}})
		return
	}
	def, ok := equity.DefaultIndexByID(id)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_INDEX", "message": "index not supported"}})
		return
	}

	interval := c.DefaultQuery("interval", "1d")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(binance.DefaultKlineLimit)))
	if limit <= 0 {
		limit = binance.DefaultKlineLimit
	}

	candles, source, err := equity.FetchCachedEastmoneyKlines(http.DefaultClient, def, interval, limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, KlineResponse{
		Symbol:   def.ID,
		Pair:     def.EastmoneySecID,
		Interval: interval,
		Candles:  candles,
		Source:   source,
	})
}

func (h *Handler) symbolAllowed(symbol string) bool {
	for _, s := range h.Config.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}
