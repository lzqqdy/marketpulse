package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/ai"
	"github.com/lzqqdy/marketpulse/internal/users"
)

func writeAIError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ai.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "ai_disabled", "message": "AI 助手未启用（需要 mysql + users + api_key）"}})
	case errors.Is(err, ai.ErrMisconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "ai_misconfigured", "message": "AI 配置不完整"}})
	case errors.Is(err, ai.ErrConversationBusy):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ai_conversation_busy", "message": "该会话正在回答中，请稍后再试"}})
	case errors.Is(err, ai.ErrQuotaExceeded):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "ai_quota_exceeded", "message": "今日 AI 额度已用完"}})
	case errors.Is(err, ai.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
	case errors.Is(err, ai.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "会话不存在"}})
	case errors.Is(err, ai.ErrUpstream):
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "ai_upstream", "message": "模型服务异常"}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "服务异常"}})
	}
}

func (h *Handler) requireAI(c *gin.Context) bool {
	if h.AI == nil || !h.AI.Enabled() {
		writeAIError(c, ai.ErrDisabled)
		return false
	}
	return true
}

func (h *Handler) aiUserID(c *gin.Context) (int64, bool) {
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

// PostAIChat handles POST /api/v1/ai/chat (SSE).
// Pre-stream errors (quota/busy/auth) return JSON; after first event uses SSE.
func (h *Handler) PostAIChat(c *gin.Context) {
	if !h.requireAI(c) {
		return
	}
	userID, ok := h.aiUserID(c)
	if !ok {
		return
	}
	var req ai.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "请求体无效"}})
		return
	}

	started := false
	flusher, canFlush := c.Writer.(http.Flusher)
	emit := func(ev ai.StreamEvent) error {
		select {
		case <-c.Request.Context().Done():
			return c.Request.Context().Err()
		default:
		}
		if !started {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			c.Status(http.StatusOK)
			started = true
		}
		payload, err := json.Marshal(ev.Data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Event, payload); err != nil {
			return err
		}
		if canFlush {
			flusher.Flush()
		}
		return nil
	}

	err := h.AI.Chat(c.Request.Context(), userID, req, emit)
	if err != nil && !started {
		writeAIError(c, err)
		return
	}
	if err != nil && started {
		if !errors.Is(err, ai.ErrUpstream) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			_ = emit(ai.StreamEvent{Event: "error", Data: map[string]string{
				"code":    "internal_error",
				"message": err.Error(),
			}})
		}
	}
}

func (h *Handler) ListAIConversations(c *gin.Context) {
	if !h.requireAI(c) {
		return
	}
	userID, ok := h.aiUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	res, err := h.AI.ListConversations(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		writeAIError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListAIMessages(c *gin.Context) {
	if !h.requireAI(c) {
		return
	}
	userID, ok := h.aiUserID(c)
	if !ok {
		return
	}
	id := c.Param("conversationId")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	includeTools := c.Query("include") == "tools"
	res, err := h.AI.ListMessages(c.Request.Context(), userID, id, limit, includeTools)
	if err != nil {
		writeAIError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) DeleteAIConversation(c *gin.Context) {
	if !h.requireAI(c) {
		return
	}
	userID, ok := h.aiUserID(c)
	if !ok {
		return
	}
	if err := h.AI.DeleteConversation(c.Request.Context(), userID, c.Param("conversationId")); err != nil {
		writeAIError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) PatchAIConversation(c *gin.Context) {
	if !h.requireAI(c) {
		return
	}
	userID, ok := h.aiUserID(c)
	if !ok {
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "请求体无效"}})
		return
	}
	conv, err := h.AI.UpdateConversationTitle(c.Request.Context(), userID, c.Param("conversationId"), body.Title)
	if err != nil {
		writeAIError(c, err)
		return
	}
	c.JSON(http.StatusOK, conv)
}
