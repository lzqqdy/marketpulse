package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/marketdata/expressnews"
)

func (h *Handler) ExpressNews(c *gin.Context) {
	tag := expressnews.NormalizeTag(c.Query("tag"))
	pn, _ := strconv.Atoi(c.DefaultQuery("pn", "0"))
	rn, _ := strconv.Atoi(c.DefaultQuery("rn", "20"))
	filterByUserStocks, _ := strconv.Atoi(c.DefaultQuery("filterByUserStocks", "0"))
	if filterByUserStocks != 0 {
		filterByUserStocks = 1
	}

	resp, err := h.MarketData.ExpressNews(tag, pn, rn, filterByUserStocks)
	if err != nil {
		writeExpressNewsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func writeExpressNewsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, expressnews.ErrInvalidTag):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "INVALID_TAG", "message": "tag must be empty, A股, 港股, 美股, or 异动"},
		})
	case errors.Is(err, expressnews.ErrInvalidPage):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "INVALID_PAGE", "message": "pn must be >= 0"},
		})
	default:
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "UPSTREAM_ERROR", "message": err.Error()},
		})
	}
}
