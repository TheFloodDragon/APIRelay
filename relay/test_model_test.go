package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apirelay/apirelay/model"
)

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
