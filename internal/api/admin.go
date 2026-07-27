package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lzqqdy/marketpulse/internal/admin"
	"github.com/lzqqdy/marketpulse/internal/users"
)

func (h *Handler) requireAdminUser(c *gin.Context) (users.User, bool) {
	if !h.requireUsers(c) {
		return users.User{}, false
	}
	if h.Admin == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden", "message": "管理员功能不可用"}})
		return users.User{}, false
	}
	user, err := h.Users.Me(c.Request.Context(), bearerToken(c))
	if err != nil {
		writeUsersError(c, err)
		return users.User{}, false
	}
	return user, true
}

// AdminMe returns whether the current user is an admin (admin.txt).
func (h *Handler) AdminMe(c *gin.Context) {
	user, ok := h.requireAdminUser(c)
	if !ok {
		return
	}
	isAdmin := h.Admin != nil && h.Admin.IsAdmin(user.Phone)
	c.JSON(http.StatusOK, gin.H{"isAdmin": isAdmin})
}

// GetAdminConfig returns live + example config and structural gaps.
func (h *Handler) GetAdminConfig(c *gin.Context) {
	user, ok := h.requireAdminUser(c)
	if !ok {
		return
	}
	if err := h.Admin.RequireAdmin(user.Phone); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden", "message": "需要管理员权限"}})
		return
	}
	view, err := h.Admin.GetConfig()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "config_read_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, view)
}

type putAdminConfigRequest struct {
	YAML string `json:"yaml"`
}

// PutAdminConfig validates and atomically writes config.yaml.
func (h *Handler) PutAdminConfig(c *gin.Context) {
	user, ok := h.requireAdminUser(c)
	if !ok {
		return
	}
	if err := h.Admin.RequireAdmin(user.Phone); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden", "message": "需要管理员权限"}})
		return
	}
	var req putAdminConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.YAML == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "yaml 不能为空"}})
		return
	}
	if err := h.Admin.SaveConfig(req.YAML); err != nil {
		if errors.Is(err, admin.ErrInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_config", "message": err.Error()}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "config_write_error", "message": "写入失败"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"restartRequired": true,
		"message":         "已保存，请重启 marketd 后生效",
	})
}
