package handler

import (
	"net/http"
	"strconv"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/gin-gonic/gin"
)

type ChannelHandler struct {
	channelService *service.ChannelService
}

func NewChannelHandler(channelService *service.ChannelService) *ChannelHandler {
	return &ChannelHandler{channelService: channelService}
}

// GetChannels 获取所有渠道
func (h *ChannelHandler) GetChannels(c *gin.Context) {
	channels, err := h.channelService.GetAllChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取渠道列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channels,
	})
}

// GetChannel 获取单个渠道
func (h *ChannelHandler) GetChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的渠道ID",
		})
		return
	}

	channel, err := h.channelService.GetChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "渠道不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channel,
	})
}

// CreateChannel 创建渠道
func (h *ChannelHandler) CreateChannel(c *gin.Context) {
	var channel model.Channel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 创建接口必须始终由数据库分配新 ID，避免前端表单残留或外部客户端
	// 传入已有 id 时触发 UNIQUE constraint failed: channels.id。
	channel.ID = 0

	if err := h.channelService.CreateChannel(&channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "创建渠道失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    channel,
	})
}

// UpdateChannel 更新渠道
func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的渠道ID",
		})
		return
	}

	var channel model.Channel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	channel.ID = uint(id)
	if err := h.channelService.UpdateChannel(&channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新渠道失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channel,
	})
}

// DeleteChannel 删除渠道
func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的渠道ID",
		})
		return
	}

	if err := h.channelService.DeleteChannel(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除渠道失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// ReorderChannels 批量更新渠道优先级
func (h *ChannelHandler) ReorderChannels(c *gin.Context) {
	var req struct {
		Orders []service.ReorderItem `json:"orders" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误: " + err.Error(),
		})
		return
	}

	if err := h.channelService.ReorderChannels(req.Orders); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新优先级失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}

// FetchModels 获取模型列表
func (h *ChannelHandler) FetchModels(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的渠道ID",
		})
		return
	}

	models, err := h.channelService.FetchModels(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取模型列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"models":  models,
	})
}

