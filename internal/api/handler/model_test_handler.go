package handler

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelTestHandler struct {
	modelTestService *service.ModelTestService
}

func NewModelTestHandler(modelTestService *service.ModelTestService) *ModelTestHandler {
	return &ModelTestHandler{modelTestService: modelTestService}
}

// GetConfig 获取模型测试全局配置。无持久化配置时返回默认值。
func (h *ModelTestHandler) GetConfig(c *gin.Context) {
	config, err := h.modelTestService.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取模型测试配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// SaveConfig 保存模型测试全局配置。
func (h *ModelTestHandler) SaveConfig(c *gin.Context) {
	var config service.ModelTestConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	saved, err := h.modelTestService.SaveConfig(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存模型测试配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    saved,
		"message": "保存成功",
	})
}
