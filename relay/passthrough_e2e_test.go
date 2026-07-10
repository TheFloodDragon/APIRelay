package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/gin-gonic/gin"
)

// anthropicOKBody 最小 Anthropic 非流式响应。
const anthropicOKBody = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-3","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`

// newRelayContextWith 构造一个携带自定义端点与请求体的 gin 上下文。
func newRelayContextWith(t *testing.T, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, err := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, rec
}

// mustMappedChannel 创建一个带上游模型映射的渠道（display -> upstream）。
func mustMappedChannel(t *testing.T, name, baseURL string, chType int, display, upstream string) *model.Channel {
	t.Helper()
	ch := &model.Channel{
		Name: name, Type: chType, Status: model.ChannelStatusEnabled,
		BaseURL: baseURL, Key: "k", Group: "default",
		Models:       display,
		ModelConfigs: `[{"name":"` + display + `","enabled":true,"upstream":"` + upstream + `"}]`,
		Priority:     10, Weight: 1,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return ch
}

// TestB1_OpenAIPassthrough_ByteDiff 同协议 OpenAI→OpenAI：上游收到的 body 与原请求
// 仅顶层 model 值不同，未知字段（tool_choice/metadata）逐字节保留。
func TestB1_OpenAIPassthrough_ByteDiff(t *testing.T) {
	setupTestDB(t)

	var captured string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer upstream.Close()

	mustMappedChannel(t, "oai", upstream.URL, constant.ChannelTypeOpenAI, "gpt-4o", "gpt-4o-real")

	// 原始请求含 IR 未建模字段 tool_choice / metadata。
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto","metadata":{"trace":"abc"}}`
	want := `{"model":"gpt-4o-real","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto","metadata":{"trace":"abc"}}`

	r := NewRelayer(&config.RelayConfig{MaxRetries: 1, DefaultGroup: "default"})
	c, rec := newRelayContextWith(t, "/v1/chat/completions", body)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if captured != want {
		t.Fatalf("passthrough body mismatch:\n got: %s\nwant: %s", captured, want)
	}
}

// TestB1_AnthropicPassthrough_ByteDiff 同协议 Anthropic→Anthropic：仅 model 值改写，
// 未知字段 thinking 保留。
func TestB1_AnthropicPassthrough_ByteDiff(t *testing.T) {
	setupTestDB(t)

	var captured string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(anthropicOKBody))
	}))
	defer upstream.Close()

	mustMappedChannel(t, "claude", upstream.URL, constant.ChannelTypeAnthropic, "claude-3", "claude-3-5-sonnet")

	body := `{"model":"claude-3","max_tokens":100,"messages":[{"role":"user","content":"hi"}],"thinking":{"type":"enabled","budget_tokens":1024}}`
	want := `{"model":"claude-3-5-sonnet","max_tokens":100,"messages":[{"role":"user","content":"hi"}],"thinking":{"type":"enabled","budget_tokens":1024}}`

	r := NewRelayer(&config.RelayConfig{MaxRetries: 1, DefaultGroup: "default"})
	c, rec := newRelayContextWith(t, "/v1/messages", body)
	r.handle(c, constant.EndpointAnthropic, apicompat.ParseAnthropicRequest)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if captured != want {
		t.Fatalf("passthrough body mismatch:\n got: %s\nwant: %s", captured, want)
	}
}

// TestB1_CrossProtocolStillRebuilds 跨协议时不透传：对外 OpenAI、上游 Anthropic，
// 上游应收到 Anthropic 结构（非原始 OpenAI body）。
func TestB1_CrossProtocolStillRebuilds(t *testing.T) {
	setupTestDB(t)

	var captured string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(anthropicOKBody))
	}))
	defer upstream.Close()

	// 对外 OpenAI，但渠道是 Anthropic 上游 → 跨协议，走 IR 重建。
	mustMappedChannel(t, "claude", upstream.URL, constant.ChannelTypeAnthropic, "gpt-4o", "claude-3-5-sonnet")

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto"}`
	r := NewRelayer(&config.RelayConfig{MaxRetries: 1, DefaultGroup: "default"})
	c, rec := newRelayContextWith(t, "/v1/chat/completions", body)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	// Anthropic 请求体必有 max_tokens 字段（BuildAnthropicRequest 强制注入），
	// 且不应是原样透传（不含 tool_choice 顶层原样）。
	if !strings.Contains(captured, `"max_tokens"`) {
		t.Fatalf("cross-protocol should rebuild to anthropic shape, got: %s", captured)
	}
	var probe dto.AnthropicRequest
	if err := json.Unmarshal([]byte(captured), &probe); err != nil {
		t.Fatalf("upstream body not valid anthropic request: %v; body=%s", err, captured)
	}
	if probe.Model != "claude-3-5-sonnet" {
		t.Errorf("rebuilt model = %q, want claude-3-5-sonnet", probe.Model)
	}
}
