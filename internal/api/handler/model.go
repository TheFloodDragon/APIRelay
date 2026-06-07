package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/apirelay/internal/repository"
)

type ModelHandler struct {
	modelRepo *repository.ModelRepository
}

func NewModelHandler(modelRepo *repository.ModelRepository) *ModelHandler {
	return &ModelHandler{modelRepo: modelRepo}
}

// GetModels 获取所有模型
func (h *ModelHandler) GetModels(c *gin.Context) {
	models, err := h.modelRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模型列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetAvailableModels 获取可用模型
func (h *ModelHandler) GetAvailableModels(c *gin.Context) {
	models, err := h.modelRepo.GetEnabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取可用模型失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// DeleteModel 删除模型
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID"})
		return
	}

	if err := h.modelRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模型失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}
