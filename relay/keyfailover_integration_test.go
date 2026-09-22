package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/apirelay/apirelay/relay/keypool"
)

// keyRotationRelayConfig 返回启用 Key 轮询、无同 Key 重试的测试配置，
// 使多 Key 用例的 Key 切换行为确定（每个 Key 失败即切换）。
func keyRotationRelayConfig(maxRetries int) *config.RelayConfig {
	cfg := &config.RelayConfig{
		MaxRetries:        maxRetries,
		ChannelMaxRetries: 0,
		CooldownSeconds:   60,
		DefaultGroup:      "default",
		KeyRotation: config.KeyRotationConfig{
			Enabled:          true,
			Strategy:         "sequential",
			KeyMaxRetries:    0,
			CooldownSeconds:  60,
			FailureThreshold: 5,
			AuthFailDisable:  true,
		},
	}
	return cfg
}

// addChannelKey 为渠道追加一个 Key（顺序追加到末尾）。
func addChannelKey(t *testing.T, channelID int, key, modelConfigs string) *model.ChannelKey {
	t.Helper()
	k := &model.ChannelKey{
		ChannelId:    channelID,
		KeyIndex:     -1, // 自动追加
		Key:          key,
		Status:       model.ChannelKeyStatusEnabled,
		ModelConfigs: modelConfigs,
	}
	if err := model.CreateChannelKey(k); err != nil {
		t.Fatalf("create channel key: %v", err)
	}
	return k
}

// resetKeypoolManager 重置 keypool 全局管理器，避免用例间内存状态串扰。
func resetKeypoolManager() {
	keypool.ResetForTest()
}

// TestKeyRotation_SwitchesToNextKeyOn401 验证渠道内某 Key 鉴权失败后切换到下一个可用 Key。
func TestKeyRotation_SwitchesToNextKeyOn401(t *testing.T) {
	setupTestDB(t)
	resetKeypoolManager()

	var badHits, goodHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch auth {
		case "Bearer sk-good":
			atomic.AddInt32(&goodHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(okBody))
		default:
			atomic.AddInt32(&badHits, 1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
		}
	}))
	defer upstream.Close()

	// 单渠道，两个 Key：第一个坏、第二个好。
	ch := &model.Channel{
		Name: "multi", Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: upstream.URL, Key: "sk-bad", Group: "default", Models: "gpt-4o", Weight: 1,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	addChannelKey(t, ch.Id, "sk-bad", "")
	addChannelKey(t, ch.Id, "sk-good", "")

	r := NewRelayer(keyRotationRelayConfig(2))
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if atomic.LoadInt32(&badHits) == 0 {
		t.Fatal("bad key should have been attempted first")
	}
	if atomic.LoadInt32(&goodHits) == 0 {
		t.Fatalf("good key should be attempted after bad key 401; status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hi") {
		t.Errorf("expected success body from good key, got %s", rec.Body.String())
	}
}

// TestKeyRotation_ExhaustsKeysThenSwitchesChannel 验证渠道内 Key 全部失败后切换到下一个渠道。
func TestKeyRotation_ExhaustsKeysThenSwitchesChannel(t *testing.T) {
	setupTestDB(t)
	resetKeypoolManager()

	var primaryHits, backupHits int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryHits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&backupHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer backup.Close()

	// primary 渠道两个 Key 都会 401；backup 渠道成功。
	primaryCh := &model.Channel{
		Name: "primary", Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: primary.URL, Key: "k", Group: "default", Models: "gpt-4o", Priority: 10, Weight: 1,
	}
	if err := model.CreateChannel(primaryCh); err != nil {
		t.Fatalf("create primary: %v", err)
	}
	addChannelKey(t, primaryCh.Id, "sk-bad-1", "")
	addChannelKey(t, primaryCh.Id, "sk-bad-2", "")

	backupCh := &model.Channel{
		Name: "backup", Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: backup.URL, Key: "k", Group: "default", Models: "gpt-4o", Priority: 1, Weight: 1,
	}
	if err := model.CreateChannel(backupCh); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	addChannelKey(t, backupCh.Id, "sk-good", "")

	r := NewRelayer(keyRotationRelayConfig(3))
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	// primary 两个 Key 各尝试一次。
	if got := atomic.LoadInt32(&primaryHits); got != 2 {
		t.Fatalf("primary hits = %d, want 2 (both keys tried)", got)
	}
	if atomic.LoadInt32(&backupHits) == 0 {
		t.Fatalf("backup channel should be attempted after primary keys exhausted; status=%d", rec.Code)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// TestKeyRotation_OverlayModelConfigIsolation 验证每个 Key 使用自己的上游模型映射，
// 切换 Key 不复用其它 Key 的模型配置（需求 4）。
func TestKeyRotation_OverlayModelConfigIsolation(t *testing.T) {
	setupTestDB(t)
	resetKeypoolManager()

	var mu sync.Mutex
	seenModels := map[string]string{} // authKey -> upstream model

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &req)
		auth := r.Header.Get("Authorization")

		mu.Lock()
		seenModels[auth] = req.Model
		mu.Unlock()

		// 第一个 Key（sk-a）返回 500 触发切换；第二个 Key（sk-b）成功。
		if auth == "Bearer sk-a" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer upstream.Close()

	ch := &model.Channel{
		Name: "iso", Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: upstream.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`, Weight: 1,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	// 两个 Key 各自映射到不同的上游模型名。
	addChannelKey(t, ch.Id, "sk-a", `[{"name":"gpt-4o","enabled":true,"upstream":"model-from-key-a"}]`)
	addChannelKey(t, ch.Id, "sk-b", `[{"name":"gpt-4o","enabled":true,"upstream":"model-from-key-b"}]`)

	r := NewRelayer(keyRotationRelayConfig(2))
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	// Key a 用自己的上游模型；Key b 用自己的上游模型；互不复用。
	if seenModels["Bearer sk-a"] != "model-from-key-a" {
		t.Errorf("key a upstream model = %q, want model-from-key-a", seenModels["Bearer sk-a"])
	}
	if seenModels["Bearer sk-b"] != "model-from-key-b" {
		t.Errorf("key b upstream model = %q, want model-from-key-b (must not reuse key a's config)", seenModels["Bearer sk-b"])
	}
}

// TestKeyRotation_QuotaExhausted429DisablesKey 验证 429 携带配额语义时，
// 该 Key 被直接失效（而非冷却重试），并切换到下一个 Key。
func TestKeyRotation_QuotaExhausted429DisablesKey(t *testing.T) {
	setupTestDB(t)
	resetKeypoolManager()

	var quotaHits, goodHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer sk-good" {
			atomic.AddInt32(&goodHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(okBody))
			return
		}
		atomic.AddInt32(&quotaHits, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"You exceeded your current quota","type":"insufficient_quota"}}`))
	}))
	defer upstream.Close()

	ch := &model.Channel{
		Name: "quota", Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: upstream.URL, Key: "k", Group: "default", Models: "gpt-4o", Weight: 1,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	quotaKey := addChannelKey(t, ch.Id, "sk-quota", "")
	addChannelKey(t, ch.Id, "sk-good", "")

	// KeyMaxRetries=2：若按普通限流处理会同 Key 重试；配额语义应直接切 Key。
	cfg := keyRotationRelayConfig(2)
	cfg.KeyRotation.KeyMaxRetries = 2
	r := NewRelayer(cfg)
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	// 配额 Key 只应被打一次（直接失效，不重试）。
	if got := atomic.LoadInt32(&quotaHits); got != 1 {
		t.Fatalf("quota key hits = %d, want 1 (should disable, not retry)", got)
	}
	if atomic.LoadInt32(&goodHits) == 0 {
		t.Fatal("good key should be attempted after quota key disabled")
	}
	// 配额 Key 应被标记为不可用并落库失效原因。
	got, err := model.GetChannelKeyByID(quotaKey.Id)
	if err != nil {
		t.Fatalf("load quota key: %v", err)
	}
	if got.Available {
		t.Errorf("quota key should be disabled (Available=false)")
	}
	if got.DisabledReason != string(model.DisableReasonQuotaExhausted) {
		t.Errorf("quota key disable reason = %q, want quota_exhausted", got.DisabledReason)
	}
}

// TestKeyRotation_FallbackVirtualKeyPreservesLegacyBehavior 验证无子表记录的旧渠道
// 仍走渠道级单 Key 逻辑（回退兼容层）：401 后切换到下一个渠道。
func TestKeyRotation_FallbackVirtualKeyPreservesLegacyBehavior(t *testing.T) {
	setupTestDB(t)
	resetKeypoolManager()

	var backupHits int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&backupHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer backup.Close()

	// 不创建任何 channel_keys 记录，走 Channel.Key 派生的虚拟单 Key。
	mustChannelURL(t, "primary", primary.URL, 10)
	mustChannelURL(t, "backup", backup.URL, 1)

	r := NewRelayer(keyRotationRelayConfig(2))
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if atomic.LoadInt32(&backupHits) == 0 {
		t.Fatalf("backup should be attempted after primary 401 (legacy fallback); status=%d", rec.Code)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
