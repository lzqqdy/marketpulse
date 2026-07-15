package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/alerts"
	"github.com/lzqqdy/marketpulse/internal/users"
)

type ackInboxRequest struct {
	DeliveryIDs []int64 `json:"deliveryIds"`
}

func writeAlertsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, alerts.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "alerts_disabled", "message": "告警模块未启用（需要 mysql + redis + users）"}})
	case errors.Is(err, alerts.ErrConditionAlreadyMet):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "alert_condition_already_met", "message": "当前已满足条件，无法保存"}})
	case errors.Is(err, alerts.ErrInvalidParams):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "alert_invalid_params", "message": "参数无效"}})
	case errors.Is(err, alerts.ErrSymbolUnavailable):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "alert_symbol_unavailable", "message": "标的暂无有效报价"}})
	case errors.Is(err, alerts.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "alert_not_found", "message": "规则不存在"}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "服务异常"}})
	}
}

func (h *Handler) requireAlerts(c *gin.Context) bool {
	if h.Alerts == nil || !h.Alerts.Enabled() {
		writeAlertsError(c, alerts.ErrDisabled)
		return false
	}
	return true
}

func (h *Handler) alertUserID(c *gin.Context) (int64, bool) {
	if h.Users == nil || !h.Users.Enabled() {
		writeUsersError(c, users.ErrDisabled)
		return 0, false
	}
	token := bearerToken(c)
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	id, err := h.Users.UserIDFromToken(c.Request.Context(), token)
	if err != nil {
		writeUsersError(c, err)
		return 0, false
	}
	return id, true
}

func (h *Handler) ListAlertRules(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	ruleType, _ := strconv.Atoi(c.Query("ruleType"))
	res, err := h.Alerts.ListRules(c.Request.Context(), userID, alerts.ListRulesQuery{
		Page:      page,
		PageSize:  pageSize,
		Status:    c.Query("status"),
		AssetType: c.Query("assetType"),
		Symbol:    c.Query("symbol"),
		RuleType:  ruleType,
		SortBy:    c.DefaultQuery("sortBy", "id"),
		SortOrder: c.DefaultQuery("sortOrder", "desc"),
	})
	if err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CreateAlertRule(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	var req alerts.CreateRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	rule, err := h.Alerts.CreateRule(c.Request.Context(), userID, req)
	if err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *Handler) UpdateAlertRule(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "无效的规则 ID"}})
		return
	}
	var req alerts.UpdateRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	rule, err := h.Alerts.UpdateRule(c.Request.Context(), userID, id, req)
	if err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *Handler) DeleteAlertRule(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "无效的规则 ID"}})
		return
	}
	if err := h.Alerts.DeleteRule(c.Request.Context(), userID, id); err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ListAlertDeliveries(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	ruleID, _ := strconv.ParseInt(c.Query("ruleId"), 10, 64)
	ruleType, _ := strconv.Atoi(c.Query("ruleType"))
	res, err := h.Alerts.ListDeliveries(c.Request.Context(), userID, alerts.ListDeliveriesQuery{
		Page:      page,
		PageSize:  pageSize,
		RuleID:    ruleID,
		Channel:   c.Query("channel"),
		Status:    c.Query("status"),
		AssetType: c.Query("assetType"),
		Symbol:    c.Query("symbol"),
		RuleType:  ruleType,
		SortBy:    c.DefaultQuery("sortBy", "createdAt"),
		SortOrder: c.DefaultQuery("sortOrder", "desc"),
	})
	if err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) AckAlertInbox(c *gin.Context) {
	if !h.requireAlerts(c) {
		return
	}
	userID, ok := h.alertUserID(c)
	if !ok {
		return
	}
	var req ackInboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	if err := h.Alerts.AckInbox(c.Request.Context(), userID, req.DeliveryIDs); err != nil {
		writeAlertsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AlertsStreamWS(c *gin.Context) {
	if h.AlertStream == nil {
		writeAlertsError(c, alerts.ErrDisabled)
		return
	}
	token := bearerToken(c)
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	h.AlertStream.ServeWS(c.Writer, c.Request, token)
}
