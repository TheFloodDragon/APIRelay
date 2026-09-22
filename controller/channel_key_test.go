package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

func setupChannelKeyControllerDB(t *testing.T) *model.Channel {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	model.DB.Exec("DELETE FROM channel_keys")
	model.DB.Exec("DELETE FROM channels")
	model.DB.Exec("DELETE FROM abilities")
	ch := &model.Channel{
		Name: "kc", Type: 1, Status: model.ChannelStatusEnabled,
		BaseURL: "https://example.test", Key: "sk-primary", Group: "default", Weight: 1,
		ModelConfigs: `[{"name":"m","enabled":true}]`,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}
	// 模拟管理端 CreateChannel handler：创建后同步首个 ChannelKey。
	model.EnsurePrimaryChannelKey(ch)
	return ch
}

func doKeyRequest(t *testing.T, method, path, body string, handler gin.HandlerFunc, params ...gin.Param) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = params
	handler(c)
	return rec
}

// 新增 Key -> 列表可见且凭据脱敏。
func TestCreateAndListChannelKey(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	id := strconv.Itoa(ch.Id)

	rec := doKeyRequest(t, http.MethodPost, "/api/channels/"+id+"/keys",
		`{"key":"sk-second","name":"second"}`, CreateChannelKey, gin.Param{Key: "id", Value: id})
	if rec.Code != http.StatusOK {
		t.Fatalf("create key status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-second") {
		t.Fatalf("response leaked key: %s", rec.Body.String())
	}

	rec = doKeyRequest(t, http.MethodGet, "/api/channels/"+id+"/keys", "", ListChannelKeys, gin.Param{Key: "id", Value: id})
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var resp struct {
		Data []struct {
			KeyMasked string `json:"key_masked"`
			HasKey    bool   `json:"has_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// 创建渠道时同步的 primary + 新增的 second。
	if len(resp.Data) != 2 {
		t.Fatalf("keys = %d, want 2 (primary synced + created)", len(resp.Data))
	}
	for _, k := range resp.Data {
		if !k.HasKey || k.KeyMasked == "" {
			t.Fatalf("key should be masked and present: %+v", k)
		}
	}
}

// 创建渠道后同步首个 ChannelKey，且幂等（重复调用不新增）。
func TestCreateChannelSyncsPrimaryKey(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	keys, err := model.ListChannelKeys(ch.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Key != "sk-primary" {
		t.Fatalf("primary key not synced on channel create: %+v", keys)
	}
	// 幂等：已有 Key 时不重复创建。
	model.EnsurePrimaryChannelKey(ch)
	if keys, _ := model.ListChannelKeys(ch.Id); len(keys) != 1 {
		t.Fatalf("EnsurePrimaryChannelKey not idempotent: %d keys", len(keys))
	}
}

// 更新 Key 留空凭据沿用旧值。
func TestUpdateChannelKeyEmptyKeepsStored(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	id := strconv.Itoa(ch.Id)
	k := &model.ChannelKey{ChannelId: ch.Id, KeyIndex: 1, Key: "sk-orig", Status: model.ChannelKeyStatusEnabled}
	if err := model.CreateChannelKey(k); err != nil {
		t.Fatal(err)
	}
	keyID := strconv.Itoa(k.Id)

	rec := doKeyRequest(t, http.MethodPut, "/api/channels/"+id+"/keys/"+keyID,
		`{"name":"renamed"}`, UpdateChannelKey,
		gin.Param{Key: "id", Value: id}, gin.Param{Key: "keyId", Value: keyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, err := model.GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "sk-orig" {
		t.Fatalf("key = %q, want unchanged sk-orig", got.Key)
	}
	if got.Name != "renamed" {
		t.Fatalf("name = %q, want renamed", got.Name)
	}
}

// 删除 Key。
func TestDeleteChannelKey(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	id := strconv.Itoa(ch.Id)
	k := &model.ChannelKey{ChannelId: ch.Id, KeyIndex: 1, Key: "sk-del", Status: model.ChannelKeyStatusEnabled}
	if err := model.CreateChannelKey(k); err != nil {
		t.Fatal(err)
	}
	keyID := strconv.Itoa(k.Id)
	rec := doKeyRequest(t, http.MethodDelete, "/api/channels/"+id+"/keys/"+keyID, "", DeleteChannelKey,
		gin.Param{Key: "id", Value: id}, gin.Param{Key: "keyId", Value: keyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if _, err := model.GetChannelKeyByID(k.Id); err == nil {
		t.Fatal("key should be deleted")
	}
}

// reorder 调整顺序。
func TestReorderChannelKeys(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	id := strconv.Itoa(ch.Id)
	// 清掉可能由其它用例（共享内存库）残留的 Key，保证本用例只面对自己建的两个。
	model.DB.Exec("DELETE FROM channel_keys")
	var ids []int
	for i := 0; i < 2; i++ {
		// KeyIndex=-1 自动追加，得到确定的 0、1 顺序。
		k := &model.ChannelKey{ChannelId: ch.Id, KeyIndex: -1, Key: "sk-" + strconv.Itoa(i), Status: model.ChannelKeyStatusEnabled}
		if err := model.CreateChannelKey(k); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, k.Id)
	}
	// 反转顺序：把后建的 Key 提到首位。
	body, _ := json.Marshal(map[string]any{"ids": []int{ids[1], ids[0]}})
	rec := doKeyRequest(t, http.MethodPost, "/api/channels/"+id+"/keys/reorder", string(body), ReorderChannelKeys,
		gin.Param{Key: "id", Value: id})
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, body=%s", rec.Code, rec.Body.String())
	}
	list, err := model.ListChannelKeys(ch.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("keys = %d, want 2; %+v", len(list), list)
	}
	if list[0].Id != ids[1] || list[1].Id != ids[0] {
		t.Fatalf("reorder did not take effect: got order [%d,%d], want [%d,%d]", list[0].Id, list[1].Id, ids[1], ids[0])
	}
}

// reset 重置失效 Key。
func TestResetChannelKeyEndpoint(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	id := strconv.Itoa(ch.Id)
	k := &model.ChannelKey{
		ChannelId: ch.Id, KeyIndex: 1, Key: "sk", Status: model.ChannelKeyStatusEnabled,
		Available: false, DisabledReason: string(model.DisableReasonAuthFailed),
	}
	if err := model.CreateChannelKey(k); err != nil {
		t.Fatal(err)
	}
	// CreateChannelKey 强制 Available=true，这里显式改回失效以模拟运行时失效。
	model.DB.Model(&model.ChannelKey{}).Where("id = ?", k.Id).
		Updates(map[string]any{"available": false, "disabled_reason": "auth_failed"})

	keyID := strconv.Itoa(k.Id)
	rec := doKeyRequest(t, http.MethodPost, "/api/channels/"+id+"/keys/"+keyID+"/reset", "", ResetChannelKey,
		gin.Param{Key: "id", Value: id}, gin.Param{Key: "keyId", Value: keyID})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, err := model.GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.DisabledReason != "" {
		t.Fatalf("key not reset: %+v", got)
	}
}

// 跨渠道访问 Key 应 404。
func TestChannelKeyWrongChannel404(t *testing.T) {
	ch := setupChannelKeyControllerDB(t)
	other := &model.Channel{Name: "other", Type: 1, Status: model.ChannelStatusEnabled, BaseURL: "https://o.test", Key: "k", Group: "default"}
	if err := model.CreateChannel(other); err != nil {
		t.Fatal(err)
	}
	k := &model.ChannelKey{ChannelId: other.Id, KeyIndex: 1, Key: "sk", Status: model.ChannelKeyStatusEnabled}
	if err := model.CreateChannelKey(k); err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(ch.Id)
	keyID := strconv.Itoa(k.Id)
	// keyId 属于 other，通过 ch 访问应 404。
	rec := doKeyRequest(t, http.MethodDelete, "/api/channels/"+id+"/keys/"+keyID, "", DeleteChannelKey,
		gin.Param{Key: "id", Value: id}, gin.Param{Key: "keyId", Value: keyID})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-channel key access status = %d, want 404", rec.Code)
	}
}
