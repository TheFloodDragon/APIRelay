package handler

import (
	"net/http"
	"strconv"

	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelHandler struct {
	modelRepo        *repository.ModelRepository
	modelTestService *service.ModelTestService
}

func NewModelHandler(modelRepo *repository.ModelRepository, modelTestService ...*service.ModelTestService) *ModelHandler {
	h := &ModelHandler{modelRepo: modelRepo}
	if len(modelTestService) > 0 {
		h.modelTestService = modelTestService[0]
	}
	return h
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

// GetModelTestChannels 获取同一调用名下的可测试渠道。
func (h *ModelHandler) GetModelTestChannels(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID"})
		return
	}
	if h.modelTestService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "模型测试服务未初始化"})
		return
	}
	channels, err := h.modelTestService.GetTestChannels(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "获取测试渠道失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channels})
}

// TestModel 对指定模型发起一次真实上游测试。
func (h *ModelHandler) TestModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID"})
		return
	}
	if h.modelTestService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "模型测试服务未初始化"})
		return
	}
	var req service.ModelTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	result, status, err := h.modelTestService.TestModel(c.Request.Context(), uint(id), req)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(status, gin.H{"success": true, "data": result})
}

func (h *ModelHandler) UpdateModel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的模型ID"})
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Enabled     *bool  `json:"enabled"`      // 使用指针以区分未传和 false
		TestEnabled *bool  `json:"test_enabled"` // 使用指针以区分未传和 false
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 获取现有模型
	model, err := h.modelRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型不存在"})
		return
	}

	// 更新字段
	displayName := req.DisplayName
	if displayName == "" {
		displayName = model.DisplayName // 保持原值
	}
	enabled := model.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	testEnabled := model.TestEnabled
	if req.TestEnabled != nil {
		testEnabled = *req.TestEnabled
	}

	if err := h.modelRepo.UpdateMetadata(uint(id), displayName, enabled, testEnabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模型失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}
