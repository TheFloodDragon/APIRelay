package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay"
	"github.com/apirelay/apirelay/relay/keypool"

	"github.com/gin-gonic/gin"
)

// channelKeyView 是 Key 的对外展示结构：掩码凭据 + 管理字段 + 运行时健康快照。
type channelKeyView struct {
	*model.ChannelKey
	// Runtime 运行时健康（keypool 内存权威优先，回退落库值）。
	Runtime channelKeyRuntimeView `json:"runtime"`
}

type channelKeyRuntimeView struct {
	Available           bool   `json:"available"`
	CooldownUntil       int64  `json:"cooldown_until"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	DisabledReason      string `json:"disabled_reason"`
	LastError           string `json:"last_error"`
	LastFailureAt       int64  `json:"last_failure_at"`
	LastSuccessAt       int64  `json:"last_success_at"`
}

// buildChannelKeyView 组装单个 Key 的展示视图（掩码凭据 + 运行时快照）。
func buildChannelKeyView(k *model.ChannelKey) channelKeyView {
	k.PrepareForAPI()
	rt := channelKeyRuntimeView{
		Available:           k.Available,
		CooldownUntil:       k.CooldownUntil,
		ConsecutiveFailures: k.ConsecutiveFailures,
		DisabledReason:      k.DisabledReason,
		LastError:           k.LastError,
		LastFailureAt:       k.LastFailureAt,
		LastSuccessAt:       k.LastSuccessAt,
	}
	// 内存状态更实时（落库是异步的），优先用它覆盖运行时展示字段。
	if snap, found := keypool.GetManager().Snapshot(k.Id); found {
		rt.Available = snap.Available
		rt.CooldownUntil = snap.CooldownUntilMs
		rt.ConsecutiveFailures = snap.ConsecutiveFailures
		rt.DisabledReason = string(snap.DisabledReason)
		rt.LastError = snap.LastError
	}
	return channelKeyView{ChannelKey: k, Runtime: rt}
}

// requireChannel 解析 :id 并确认渠道存在。
func requireChannel(c *gin.Context) (*model.Channel, bool) {
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelByID(id)
	if err != nil {
		fail(c, http.StatusNotFound, "供应商不存在")
		return nil, false
	}
	return ch, true
}

// requireChannelKey 在已取得渠道的前提下解析并校验 Key 归属该渠道。
func requireChannelKey(c *gin.Context, ch *model.Channel) (*model.ChannelKey, bool) {
	keyID, _ := strconv.Atoi(c.Param("keyId"))
	k, err := model.GetChannelKeyByID(keyID)
	if err != nil || k.ChannelId != ch.Id {
		fail(c, http.StatusNotFound, "Key 不存在")
		return nil, false
	}
	return k, true
}

// validateKeyModelConfigs 校验 Key 级模型配置为合法 JSON（空值合法）。
func validateKeyModelConfigs(c *gin.Context, k *model.ChannelKey) bool {
	if strings.TrimSpace(k.ModelConfigs) == "" {
		return true
	}
	if list := k.ModelConfigList(); list == nil {
		fail(c, http.StatusBadRequest, "model_configs 必须是合法的 JSON 数组")
		return false
	}
	return true
}

// ListChannelKeys GET /api/channels/:id/keys
// 列出渠道所有 Key（掩码凭据 + 运行时健康）。
func ListChannelKeys(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	keys, err := model.ListChannelKeys(ch.Id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]channelKeyView, 0, len(keys))
	for _, k := range keys {
		views = append(views, buildChannelKeyView(k))
	}
	ok(c, views)
}

// CreateChannelKey POST /api/channels/:id/keys
// 为渠道新增一个 Key。凭据必填；KeyIndex 自动追加到末尾。
func CreateChannelKey(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	var in model.ChannelKey
	if !bindJSON(c, &in) {
		return
	}
	in.ChannelId = ch.Id
	in.KeyIndex = -1 // 自动追加
	in.ApplySubmittedKey(nil)
	if strings.TrimSpace(in.Key) == "" {
		fail(c, http.StatusBadRequest, "API Key 不能为空")
		return
	}
	if !validateKeyModelConfigs(c, &in) {
		return
	}
	if err := model.CreateChannelKey(&in); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, buildChannelKeyView(&in))
}

// UpdateChannelKey PUT /api/channels/:id/keys/:keyId
// 更新 Key 的备注/凭据/状态/模型配置。凭据留空表示沿用已保存值。
func UpdateChannelKey(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	existing, found := requireChannelKey(c, ch)
	if !found {
		return
	}
	var in model.ChannelKey
	if !bindJSON(c, &in) {
		return
	}
	// 编辑表单不回显明文凭据：提交为空表示沿用，非空才覆盖。
	in.ApplySubmittedKey(existing)
	if strings.TrimSpace(in.Key) == "" {
		fail(c, http.StatusBadRequest, "API Key 不能为空")
		return
	}
	if !validateKeyModelConfigs(c, &in) {
		return
	}
	// 固定不可由请求体篡改的字段。
	in.Id = existing.Id
	in.ChannelId = existing.ChannelId
	in.KeyIndex = existing.KeyIndex // 顺序调整走独立的 reorder 接口
	if in.Status == 0 {
		in.Status = existing.Status
	}
	if err := model.UpdateChannelKey(&in); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	updated, err := model.GetChannelKeyByID(in.Id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, buildChannelKeyView(updated))
}

// DeleteChannelKey DELETE /api/channels/:id/keys/:keyId
func DeleteChannelKey(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	k, found := requireChannelKey(c, ch)
	if !found {
		return
	}
	if err := model.DeleteChannelKey(k.Id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 丢弃内存运行时状态，避免 keypool 泄漏已删除 Key。
	keypool.GetManager().Forget(k.Id)
	ok(c, gin.H{"deleted": k.Id})
}

// ReorderChannelKeys POST /api/channels/:id/keys/reorder  body: {"ids":[...]}
// 按给定顺序重排 KeyIndex（首位最高优先，顺序优先策略据此选 Key）。
func ReorderChannelKeys(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
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
	if err := model.ReorderChannelKeys(ch.Id, req.IDs); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"reordered": len(req.IDs)})
}

// ResetChannelKey POST /api/channels/:id/keys/:keyId/reset
// 手动重置 Key 运行时健康（清冷却/失效/失败计数），重新入轮询队列。
func ResetChannelKey(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	k, found := requireChannelKey(c, ch)
	if !found {
		return
	}
	if err := model.ResetChannelKey(k.Id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 同步重置内存运行时状态，避免落库值被在途旧快照覆盖。
	fresh, err := model.GetChannelKeyByID(k.Id)
	if err == nil {
		keypool.GetManager().MarkRecovered(fresh)
	}
	ok(c, gin.H{"reset": k.Id})
}

// TestChannelKey POST /api/channels/:id/keys/:keyId/test  body: {"model":"...","prompt":"..."}
// 用指定 Key 的 overlay 视图对某模型发起连通性测试。
func TestChannelKey(c *gin.Context) {
	ch, found := requireChannel(c)
	if !found {
		return
	}
	k, found := requireChannelKey(c, ch)
	if !found {
		return
	}
	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if !bindJSON(c, &req) {
		return
	}
	displayModel := strings.TrimSpace(req.Model)
	if displayModel == "" {
		// 未指定时取渠道首个启用模型。
		names := ch.EnabledModelNames()
		if len(names) == 0 {
			fail(c, http.StatusBadRequest, "请指定要测试的模型")
			return
		}
		displayModel = names[0]
	}
	// 用 Key overlay 视图测试：走该 Key 自己的凭据与模型配置（需求 4）。
	overlay := model.BuildKeyOverlay(ch, k)
	res := relay.TestModelWithPrompt(c.Request.Context(), overlay, displayModel, req.Prompt)
	ok(c, res)
}
