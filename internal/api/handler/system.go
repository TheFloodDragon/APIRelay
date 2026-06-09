package handler

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	settingsService *service.SettingsService
}

func NewSystemHandler(settingsService ...*service.SettingsService) *SystemHandler {
	h := &SystemHandler{}
	if len(settingsService) > 0 {
		h.settingsService = settingsService[0]
	}
	return h
}

// Health 健康检查
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "APIRelay is running",
	})
}

// Info 系统信息
func (h *SystemHandler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":        "APIRelay",
		"version":     "1.0.0",
		"description": "API调度中心",
	})
}

// GetSettings 获取全局设置。
func (h *SystemHandler) GetSettings(c *gin.Context) {
	if h.settingsService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置服务未初始化"})
		return
	}
	settings, err := h.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取设置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings})
}

// UpdateSettings 更新全局设置。
func (h *SystemHandler) UpdateSettings(c *gin.Context) {
	if h.settingsService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置服务未初始化"})
		return
	}
	var req service.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	settings, err := h.settingsService.UpdateSettings(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings, "message": "更新成功"})
}
