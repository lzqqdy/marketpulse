package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/portfolio"
	"github.com/lzqqdy/marketpulse/internal/users"
)

func writePortfolioError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, portfolio.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "portfolio_disabled", "message": "资产中心未启用（需要 mysql + users）"}})
	case errors.Is(err, portfolio.ErrSymbolUnavailable):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "symbol_unavailable", "message": err.Error()}})
	case errors.Is(err, portfolio.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": err.Error()}})
	case errors.Is(err, portfolio.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "资源不存在"}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "服务异常"}})
	}
}

func (h *Handler) requirePortfolio(c *gin.Context) bool {
	if h.Portfolio == nil || !h.Portfolio.Enabled() {
		writePortfolioError(c, portfolio.ErrDisabled)
		return false
	}
	return true
}

func (h *Handler) portfolioUserID(c *gin.Context) (int64, bool) {
	if h.Users == nil || !h.Users.Enabled() {
		writeUsersError(c, users.ErrDisabled)
		return 0, false
	}
	token := bearerToken(c)
	id, err := h.Users.UserIDFromToken(c.Request.Context(), token)
	if err != nil {
		writeUsersError(c, err)
		return 0, false
	}
	return id, true
}

func (h *Handler) GetPortfolioHoldings(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	res, err := h.Portfolio.GetHoldings(c.Request.Context(), userID)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) PutPortfolioHoldings(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	var req portfolio.PutHoldingsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	res, err := h.Portfolio.PutHoldings(c.Request.Context(), userID, req)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) PutPortfolioSettings(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	var req portfolio.PutSettingsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	res, err := h.Portfolio.PutSettings(c.Request.Context(), userID, req)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) GetPortfolioOverview(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	res, err := h.Portfolio.Overview(c.Request.Context(), userID)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListPortfolioSnapshots(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	res, err := h.Portfolio.ListSnapshots(c.Request.Context(), userID, portfolio.ListSnapshotsQuery{
		Page:      page,
		PageSize:  pageSize,
		From:      c.Query("from"),
		To:        c.Query("to"),
		SortBy:    c.DefaultQuery("sort", "date"),
		SortOrder: c.DefaultQuery("order", "desc"),
	})
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) GetPortfolioEligibleSymbols(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	if _, ok := h.portfolioUserID(c); !ok {
		return
	}
	res, err := h.Portfolio.EligibleSymbols(c.Request.Context())
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) GetPortfolioReportSeries(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	res, err := h.Portfolio.ReportSeries(c.Request.Context(), userID, c.DefaultQuery("range", "30d"))
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) GetPortfolioReportAllocation(c *gin.Context) {
	if !h.requirePortfolio(c) {
		return
	}
	userID, ok := h.portfolioUserID(c)
	if !ok {
		return
	}
	res, err := h.Portfolio.ReportAllocation(c.Request.Context(), userID)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
