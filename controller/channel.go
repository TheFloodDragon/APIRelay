package controller

import (
	"net/http"
	"strconv"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay"

	"github.com/gin-gonic/gin"
)

// ListChannels GET /api/channels
func ListChannels(c *gin.Context) {
	list, err := model.ListChannels()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, list)
}

// CreateChannel POST /api/channels
func CreateChannel(c *gin.Context) {
	var ch model.Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if ch.Status == 0 {
		ch.Status = model.ChannelStatusEnabled
	}
	if ch.Group == "" {
		ch.Group = "default"
	}
	if ch.Weight == 0 {
		ch.Weight = 1
	}
	if err := model.CreateChannel(&ch); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, ch)
}

// UpdateChannel PUT /api/channels/:id
func UpdateChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	existing, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "供应商不存在")
		return
	}
	var in model.Channel
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	in.Id = existing.Id
	in.CreatedAt = existing.CreatedAt
	// 密钥留空表示不修改，保留原有密钥
	if in.Key == "" {
		in.Key = existing.Key
	}
	if err := model.UpdateChannel(&in); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, in)
}

// DeleteChannel DELETE /api/channels/:id
func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteChannel(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": id})
}

// ReorderChannels POST /api/channels/reorder
// 按给定 ID 顺序重排供应商优先级（首位最高）。
func ReorderChannels(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.IDs) == 0 {
		fail(c, http.StatusBadRequest, "ids 不能为空")
		return
	}
	if err := model.ReorderChannels(req.IDs); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"reordered": len(req.IDs)})
}

// ProbeChannelModels GET /api/channels/:id/models
// 按已保存渠道的协议调用上游标准模型列表接口，返回模型 ID 列表。
func ProbeChannelModels(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "供应商不存在")
		return
	}
	models, err := relay.ProbeModels(ch)
	if err != nil {
		fail(c, http.StatusBadGateway, "拉取模型失败: "+err.Error())
		return
	}
	ok(c, gin.H{"models": models})
}

// ProbeModelsByConfig POST /api/channels/probe-models
// 在创建渠道前，按临时填写的 base_url/key/type 探测模型列表（无需先保存）。
func ProbeModelsByConfig(c *gin.Context) {
	var in model.Channel
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	models, err := relay.ProbeModels(&in)
	if err != nil {
		fail(c, http.StatusBadGateway, "拉取模型失败: "+err.Error())
		return
	}
	ok(c, gin.H{"models": models})
}

// ChannelTypes GET /api/channel-types 返回支持的渠道协议类型。
func ChannelTypes(c *gin.Context) {
	types := []gin.H{
		{"value": constant.ChannelTypeOpenAI, "name": constant.ChannelTypeName(constant.ChannelTypeOpenAI), "default_base_url": "https://api.openai.com"},
		{"value": constant.ChannelTypeAnthropic, "name": constant.ChannelTypeName(constant.ChannelTypeAnthropic), "default_base_url": "https://api.anthropic.com"},
		{"value": constant.ChannelTypeResponses, "name": constant.ChannelTypeName(constant.ChannelTypeResponses), "default_base_url": "https://api.openai.com"},
	}
	ok(c, types)
}
