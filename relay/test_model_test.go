package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
