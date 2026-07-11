package relay

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/gin-gonic/gin"
)

// okBody 是一个最小 OpenAI Chat 响应体。
const okBody = `{"id":"cmpl-1","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

// newRelayTestContext 构造一个携带 OpenAI Chat 请求体的 gin 上下文。
func newRelayTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, rec
}

// mustChannelURL 创建一个指向指定 baseURL 的 OpenAI 渠道。
func mustChannelURL(t *testing.T, name, baseURL string, priority int) *model.Channel {
	t.Helper()
	ch := &model.Channel{
		Name: name, Type: constant.ChannelTypeOpenAI, Status: model.ChannelStatusEnabled,
		BaseURL: baseURL, Key: "k", Group: "default",
		Models: "gpt-4o", Priority: priority, Weight: 1,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return ch
}

// TestFailover_SwitchesToSecondChannelOn502 覆盖 A5（502 触发切换）与 A9
// （MaxRetries=1 语义：首选失败后仍尝试第二个渠道）。
func TestFailover_SwitchesToSecondChannelOn502(t *testing.T) {
	setupTestDB(t)

	var primaryHits, backupHits int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryHits, 1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"message":"bad gateway"}}`))
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&backupHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okBody))
	}))
	defer backup.Close()

	// primary 优先级更高，会先被选中；backup 作为故障转移目标。
	mustChannelURL(t, "primary", primary.URL, 10)
	mustChannelURL(t, "backup", backup.URL, 1)

	// MaxRetries=1：若存在 off-by-one，则 primary 502 后不会尝试 backup。
	r := NewRelayer(&config.RelayConfig{MaxRetries: 1, ChannelMaxRetries: 0, DefaultGroup: "default"})
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if atomic.LoadInt32(&primaryHits) == 0 {
		t.Fatal("primary channel should have been attempted")
	}
	if atomic.LoadInt32(&backupHits) == 0 {
		t.Fatalf("backup channel should be attempted after primary 502 (off-by-one?); status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hi") {
		t.Errorf("expected backup success body, got %s", rec.Body.String())
	}
}

// TestFailover_401TriggersSwitch 覆盖 A5：渠道级 401 应切换到下一个渠道而非直接判致命。
func TestFailover_401TriggersSwitch(t *testing.T) {
	setupTestDB(t)

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

	mustChannelURL(t, "primary", primary.URL, 10)
	mustChannelURL(t, "backup", backup.URL, 1)

	r := NewRelayer(&config.RelayConfig{MaxRetries: 2, ChannelMaxRetries: 0, DefaultGroup: "default"})
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if atomic.LoadInt32(&backupHits) == 0 {
		t.Fatalf("backup should be attempted after primary 401; status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("final status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// TestFailover_ExhaustedReturnsRealStatus 覆盖：所有渠道 401 耗尽后返回真实状态码而非伪装 200。
func TestFailover_ExhaustedReturnsRealStatus(t *testing.T) {
	setupTestDB(t)

	unauthorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	})
	s1 := httptest.NewServer(unauthorized)
	defer s1.Close()
	s2 := httptest.NewServer(unauthorized)
	defer s2.Close()

	mustChannelURL(t, "c1", s1.URL, 10)
	mustChannelURL(t, "c2", s2.URL, 5)

	r := NewRelayer(&config.RelayConfig{MaxRetries: 3, ChannelMaxRetries: 0, DefaultGroup: "default"})
	c, rec := newRelayTestContext(t)
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("exhausted status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code == http.StatusOK {
		t.Fatal("must not disguise failure as 200")
	}
	_ = fmt.Sprint(rec.Body.String())
}

func TestFailover_StreamInterruptedAfterWriteDoesNotFinishOrSwitch(t *testing.T) {
	setupTestDB(t)

	var primaryHits, backupHits int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryHits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-real\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// 不发送 [DONE] 就关闭响应，模拟首包后的上游断流。
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&backupHits, 1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"backup\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer backup.Close()

	mustChannelURL(t, "primary", primary.URL, 10)
	mustChannelURL(t, "backup", backup.URL, 1)

	r := NewRelayer(&config.RelayConfig{MaxRetries: 2, ChannelMaxRetries: 1, DefaultGroup: "default"})
	c, rec := newRelayTestContext(t)
	c.Request.Body = io.NopCloser(strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	r.handle(c, constant.EndpointOpenAI, apicompat.ParseOpenAIRequest)

	if atomic.LoadInt32(&primaryHits) != 1 {
		t.Fatalf("primary hits = %d, want 1", atomic.LoadInt32(&primaryHits))
	}
	if atomic.LoadInt32(&backupHits) != 0 {
		t.Fatalf("backup must not be attempted after response started; hits=%d", atomic.LoadInt32(&backupHits))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "partial") {
		t.Fatalf("partial upstream data should be preserved, body=%s", body)
	}
	if strings.Contains(body, "[DONE]") {
		t.Fatalf("interrupted stream must not receive synthetic [DONE], body=%s", body)
	}
	if strings.Contains(body, "backup") {
		t.Fatalf("backup response must not be appended, body=%s", body)
	}
	if strings.Contains(body, "gpt-4o-real") || !strings.Contains(body, `"model":"gpt-4o"`) {
		t.Fatalf("raw stream must expose the display model instead of upstream model, body=%s", body)
	}
}
