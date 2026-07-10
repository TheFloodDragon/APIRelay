package controller

import (
	"net/http"
	"strconv"
	"strings"

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

func validateOverrides(c *gin.Context, ch *model.Channel) bool {
	if _, err := ch.ParseHeaderOverride(); err != nil {
		fail(c, http.StatusBadRequest, "header_override: "+err.Error())
		return false
	}
	if _, err := ch.ParseBodyOverride(); err != nil {
		fail(c, http.StatusBadRequest, "body_override: "+err.Error())
		return false
	}
	return true
}

// CreateChannel POST /api/channels
func CreateChannel(c *gin.Context) {
	var ch model.Channel
	if !bindJSON(c, &ch) {
		return
	}
	if !validateOverrides(c, &ch) {
		return
	}
	if strings.TrimSpace(ch.Key) == "" {
		fail(c, http.StatusBadRequest, "API Key 不能为空")
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
	if !bindJSON(c, &in) {
		return
	}
	if !validateOverrides(c, &in) {
		return
	}
	if strings.TrimSpace(in.Key) == "" {
		fail(c, http.StatusBadRequest, "API Key 不能为空")
		return
	}
	in.Id = existing.Id
	in.CreatedAt = existing.CreatedAt
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
	if !bindJSON(c, &req) {
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
	if !validateOverrides(c, ch) {
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
	if !bindJSON(c, &in) {
		return
	}
	if !validateOverrides(c, &in) {
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

// TestChannelModel POST /api/channels/:id/test  body: {"model":"..."}
// 对已保存渠道的指定模型发起连通性测试。
func TestChannelModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "供应商不存在")
		return
	}
	if !validateOverrides(c, ch) {
		return
	}
	var req struct {
		Model string `json:"model"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Model == "" {
		fail(c, http.StatusBadRequest, "缺少 model")
		return
	}
	ok(c, relay.TestModel(ch, req.Model))
}

// TestChannelByConfig POST /api/channels/test
// 用临时配置（未保存）测试模型连通性。body 为渠道配置 + model 字段。
func TestChannelByConfig(c *gin.Context) {
	var req struct {
		model.Channel
		Model string `json:"model"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ch := req.Channel
	if !validateOverrides(c, &ch) {
		return
	}
	if req.Model == "" {
		fail(c, http.StatusBadRequest, "缺少 model")
		return
	}
	ok(c, relay.TestModel(&ch, req.Model))
}

// batchTestSummary 批量测试汇总。
type batchTestSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// summarizeResults 统计批量测试结果。
func summarizeResults(results []*relay.ModelTestResult) batchTestSummary {
	s := batchTestSummary{Total: len(results)}
	for _, r := range results {
		if r != nil && r.Success {
			s.Success++
		} else {
			s.Failed++
		}
	}
	return s
}

// TestChannelAllModels POST /api/channels/:id/test-all
// 对已保存渠道批量测试模型连通性。默认取该渠道的全部启用模型；
// body 可选 {"models":[...]} 覆盖测试范围。
func TestChannelAllModels(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "供应商不存在")
		return
	}
	if !validateOverrides(c, ch) {
		return
	}
	var req struct {
		Models []string `json:"models"`
	}
	// body 可选，绑定失败不阻断（使用默认启用模型）。
	_ = c.ShouldBindJSON(&req)

	models := req.Models
	if len(models) == 0 {
		models = ch.EnabledModelNames()
	}
	if len(models) == 0 {
		fail(c, http.StatusBadRequest, "该渠道没有可测试的启用模型")
		return
	}

	results := relay.TestModels(c.Request.Context(), ch, models, 0)
	ok(c, gin.H{"results": results, "summary": summarizeResults(results)})
}

// TestChannelBatchByConfig POST /api/channels/test-batch
// 用临时配置（未保存）批量测试模型连通性。
// body 为渠道配置 + {"models":[...]}。
func TestChannelBatchByConfig(c *gin.Context) {
	var req struct {
		model.Channel
		Models []string `json:"models"`
	}
	if !bindJSON(c, &req) {
		return
	}
	ch := req.Channel
	if !validateOverrides(c, &ch) {
		return
	}
	if len(req.Models) == 0 {
		fail(c, http.StatusBadRequest, "缺少 models")
		return
	}
	results := relay.TestModels(c.Request.Context(), &ch, req.Models, 0)
	ok(c, gin.H{"results": results, "summary": summarizeResults(results)})
}
