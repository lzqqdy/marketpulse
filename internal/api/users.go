package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/users"
)

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func writeUsersError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, users.ErrDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": "users_disabled", "message": "用户模块未启用（需要 mysql + redis）"}})
	case errors.Is(err, users.ErrRateLimited), errors.Is(err, users.ErrLoginLocked):
		sec := users.DenyRetryAfterSeconds(err)
		c.Header("Retry-After", strconv.Itoa(sec))
		code := "rate_limited"
		if errors.Is(err, users.ErrLoginLocked) {
			code = "login_locked"
		}
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": code, "message": users.DenyMessage(err)}})
	case errors.Is(err, users.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "invalid_credentials", "message": "手机号或密码错误"}})
	case errors.Is(err, users.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "请先登录"}})
	case errors.Is(err, users.ErrWrongPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "wrong_password", "message": "原密码不正确"}})
	case errors.Is(err, users.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_input", "message": err.Error()}})
	case errors.Is(err, users.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "用户不存在"}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "服务异常"}})
	}
}

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(c.GetHeader("X-Session-Token"))
}

func (h *Handler) requireUsers(c *gin.Context) bool {
	if h.Users == nil || !h.Users.Enabled() {
		writeUsersError(c, users.ErrDisabled)
		return false
	}
	return true
}

func (h *Handler) Login(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	res, err := h.Users.Login(c.Request.Context(), req.Phone, req.Password, c.ClientIP())
	if err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Logout(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	_ = h.Users.Logout(c.Request.Context(), bearerToken(c))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) Me(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	user, err := h.Users.Me(c.Request.Context(), bearerToken(c))
	if err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	var req users.UpdateProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	user, err := h.Users.UpdateProfile(c.Request.Context(), bearerToken(c), req)
	if err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请求体无效"}})
		return
	}
	if err := h.Users.ChangePassword(c.Request.Context(), bearerToken(c), req.OldPassword, req.NewPassword); err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) UploadAvatar(c *gin.Context) {
	if !h.requireUsers(c) {
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "请选择图片文件（字段名 file）"}})
		return
	}
	user, err := h.Users.UploadAvatar(c.Request.Context(), bearerToken(c), fh)
	if err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}
