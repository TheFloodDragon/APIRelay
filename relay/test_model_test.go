package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apirelay/apirelay/model"
)

func assertOverrideHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("X-Custom-Trace"); got != "trace-1" {
		t.Errorf("X-Custom-Trace = %q, want trace-1", got)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer k" {
		t.Errorf("Authorization = %q, want Bearer k", got)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if r.Host == "evil.example" {
		t.Errorf("dangerous Host override was applied")
	}
}

func TestTestModel_Success(t *testing.T) {
	// 模拟 OpenAI 上游
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"c1","object":"chat.completion","model":"m",
			"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "test", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`,
	}
	res := TestModel(ch, "gpt-4o")
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
	if res.Reply != "hi" {
		t.Errorf("reply = %q, want hi", res.Reply)
	}
	if res.Protocol != "openai" {
		t.Errorf("protocol = %q, want openai", res.Protocol)
	}
	if res.Usage == nil || res.Usage.TotalTokens != 6 {
		t.Errorf("usage = %+v", res.Usage)
	}
}

func TestTestModel_HeaderOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertOverrideHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "headers", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`,
		HeaderOverride: `{
			"X-Custom-Trace":"trace-1",
			"Authorization":"Bearer bad",
			"Content-Type":"text/plain",
			"Host":"evil.example"
		}`,
	}
	res := TestModel(ch, "gpt-4o")
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
}

func TestTestModel_AnthropicProtocolHeadersCannotBeOverridden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "k" {
			t.Errorf("X-Api-Key = %q, want k", got)
		}
		if got := r.Header.Get("Anthropic-Version"); got != "2023-06-01" {
			t.Errorf("Anthropic-Version = %q, want 2023-06-01", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude","stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "anthropic-headers", Type: 2, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs:   `[{"name":"claude","enabled":true}]`,
		HeaderOverride: `{"x-api-key":"bad","anthropic-version":"2024-01-01"}`,
	}
	if res := TestModel(ch, "claude"); !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
}

func TestTestModel_EmptyHeaderOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("Authorization = %q, want Bearer k", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "empty-headers", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`, HeaderOverride: "  ",
	}
	if res := TestModel(ch, "gpt-4o"); !res.Success {
		t.Fatalf("empty header override should not fail: %s", res.Error)
	}
}

func TestProbeModels_HeaderOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Custom-Trace"); got != "trace-1" {
			t.Errorf("X-Custom-Trace = %q, want trace-1", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("Authorization = %q, want Bearer k", got)
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want empty", got)
		}
		if r.Host == "evil.example" {
			t.Errorf("dangerous Host override was applied")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Type: 1, BaseURL: srv.URL, Key: "k",
		HeaderOverride: `{
			"X-Custom-Trace":"trace-1",
			"Authorization":"Bearer bad",
			"Content-Type":"text/plain",
			"Host":"evil.example"
		}`,
	}
	models, err := ProbeModels(ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0] != "gpt-4o" {
		t.Fatalf("models = %v", models)
	}
}

func TestProbeModels_RetriesBearerForAnthropicCompatibleAggregator(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			if got := r.Header.Get("x-api-key"); got != "k" {
				t.Errorf("first x-api-key = %q, want k", got)
			}
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("first Authorization = %q, want empty", got)
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"missing token"}}`))
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("retry Authorization = %q, want Bearer k", got)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Errorf("retry x-api-key = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-compatible"}]}`))
	}))
	defer srv.Close()

	models, err := ProbeModels(&model.Channel{Type: 2, BaseURL: srv.URL, Key: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
	if len(models) != 1 || models[0] != "claude-compatible" {
		t.Fatalf("models = %v", models)
	}
}

func TestTestModel_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "bad", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`,
	}
	res := TestModel(ch, "gpt-4o")
	if res.Success {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Error, "401") || !strings.Contains(res.Error, "Invalid API key") {
		t.Errorf("error should mention status and upstream message: %q", res.Error)
	}
}

func TestTestModel_NilChannel(t *testing.T) {
	res := TestModel(nil, "gpt-4o")
	if res.Success || res.Error == "" {
		t.Error("nil channel should fail with error")
	}
}

func TestTestModel_UpstreamModelMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "map", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4","enabled":true,"upstream":"gpt-4o-real"}]`,
	}
	res := TestModel(ch, "gpt-4")
	if !res.Success {
		t.Fatalf("expected success: %s", res.Error)
	}
	if res.Upstream != "gpt-4o-real" {
		t.Errorf("upstream = %q, want gpt-4o-real", res.Upstream)
	}
}

// TestTestModels_Batch 批量测试：多模型并发、结果按输入顺序返回、混合成功/失败。
func TestTestModels_Batch(t *testing.T) {
	// 上游：good-* 模型返回 200，其余返回 401。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		if strings.Contains(string(body), "bad") {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"message":"nope"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "batch", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"good-1","enabled":true},{"name":"bad-1","enabled":true},{"name":"good-2","enabled":true}]`,
	}
	models := []string{"good-1", "bad-1", "good-2"}
	results := TestModels(context.Background(), ch, models, 5)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// 顺序与输入一致
	for i, m := range models {
		if results[i] == nil || results[i].Model != m {
			t.Fatalf("result[%d] model mismatch: %+v (want %s)", i, results[i], m)
		}
	}
	if !results[0].Success || results[1].Success || !results[2].Success {
		t.Errorf("success pattern = [%v %v %v], want [true false true]",
			results[0].Success, results[1].Success, results[2].Success)
	}
}

// TestTestModels_Empty 空模型列表返回空结果。
func TestTestModels_HeaderOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertOverrideHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "batch-headers", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"one","enabled":true},{"name":"two","enabled":true}]`,
		HeaderOverride: `{
			"X-Custom-Trace":"trace-1",
			"Authorization":"Bearer bad",
			"Content-Type":"text/plain",
			"Host":"evil.example"
		}`,
	}
	results := TestModels(context.Background(), ch, []string{"one", "two"}, 2)
	for i, result := range results {
		if result == nil || !result.Success {
			t.Fatalf("result[%d] = %+v", i, result)
		}
	}
}

func TestTestModels_Empty(t *testing.T) {
	ch := &model.Channel{Name: "e", Type: 1, BaseURL: "http://x", Key: "k"}
	results := TestModels(context.Background(), ch, nil, 5)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

// TestTestModelWithContext_Timeout 慢上游触发 context 超时。
func TestTestModelWithContext_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"late"}}],"usage":{}}`))
	}))
	defer srv.Close()

	ch := &model.Channel{
		Name: "slow", Type: 1, BaseURL: srv.URL, Key: "k", Group: "default",
		ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res := TestModelWithContext(ctx, ch, "gpt-4o")
	if res.Success {
		t.Fatal("expected failure due to timeout")
	}
	if res.Error == "" {
		t.Error("expected an error message on timeout")
	}
}
