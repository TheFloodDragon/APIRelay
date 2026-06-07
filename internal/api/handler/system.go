package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
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
