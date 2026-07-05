package relay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/model"
)

func TestExtractUpstreamErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"openai", `{"error":{"message":"Invalid API key","type":"auth"}}`, "Invalid API key"},
		{"anthropic", `{"type":"error","error":{"type":"not_found_error","message":"model not found"}}`, "model not found"},
		{"error_string", `{"error":"bad gateway"}`, "bad gateway"},
		{"top_message", `{"message":"rate limited"}`, "rate limited"},
		{"plain_text", `Service Unavailable`, "Service Unavailable"},
		{"empty", ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractUpstreamErrorMessage([]byte(c.body)); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestStripInternalPrefix(t *testing.T) {
	cases := map[string]string{
		"upstream status 502: bad gateway":               "bad gateway",
		"stream: connection reset":                       "connection reset",
		"do request: dial tcp timeout":                   "dial tcp timeout",
		`upstream status 401: {"error":{"message":"x"}}`: `{"error":{"message":"x"}}`,
		"plain error": "plain error",
	}
	for in, want := range cases {
		if got := stripInternalPrefix(in); got != want {
			t.Errorf("stripInternalPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFriendlyUpstreamError_EmptyResponse(t *testing.T) {
	info := &RelayInfo{OriginModel: "gpt-4o", UpstreamModel: "gpt-4o-real"}
	msg := friendlyUpstreamError(info, http.StatusBadGateway, errEmptyUpstreamResponse)
	if !strings.Contains(msg, "空响应") || !strings.Contains(msg, "gpt-4o-real") {
		t.Errorf("empty response hint missing context: %q", msg)
	}
	// 不应泄漏英文 sentinel
	if strings.Contains(msg, "empty upstream response") {
		t.Errorf("internal sentinel leaked: %q", msg)
	}
}

func TestFriendlyUpstreamError_StatusAndDetail(t *testing.T) {
	info := &RelayInfo{OriginModel: "claude-3"}
	err := fmt.Errorf("upstream status 401: %s", `{"error":{"message":"Invalid key"}}`)
	msg := friendlyUpstreamError(info, http.StatusUnauthorized, err)
	if !strings.Contains(msg, "鉴权失败") {
		t.Errorf("missing auth hint: %q", msg)
	}
	if !strings.Contains(msg, "Invalid key") {
		t.Errorf("missing upstream detail: %q", msg)
	}
	// 不应泄漏内部前缀
	if strings.Contains(msg, "upstream status") {
		t.Errorf("internal prefix leaked: %q", msg)
	}
}

func TestFriendlyExhaustedError(t *testing.T) {
	info := &RelayInfo{OriginModel: "gpt-4o"}
	// 空响应场景
	if msg := friendlyExhaustedError(info, http.StatusBadGateway, errEmptyUpstreamResponse.Error()); !strings.Contains(msg, "空响应") {
		t.Errorf("exhausted empty-response hint missing: %q", msg)
	}
	// 通用场景
	msg := friendlyExhaustedError(info, http.StatusServiceUnavailable, "upstream status 503: overloaded")
	if !strings.Contains(msg, "所有可用渠道均请求失败") || !strings.Contains(msg, "overloaded") {
		t.Errorf("exhausted message missing parts: %q", msg)
	}
}

func TestStatusHint(t *testing.T) {
	if !strings.Contains(statusHint(http.StatusTooManyRequests), "限流") {
		t.Error("429 hint should mention 限流")
	}
	if !strings.Contains(statusHint(http.StatusUnauthorized), "鉴权") {
		t.Error("401 hint should mention 鉴权")
	}
}

func TestTruncateMessage(t *testing.T) {
	if got := truncateMessage("abcdef", 3); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncateMessage("ab", 5); got != "ab" {
		t.Errorf("no-truncate = %q", got)
	}
}

func TestClassifyRelayError(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got := classifyRelayError(canceled, errors.New("anything")); got != ErrorCategoryClientCanceled {
		t.Fatalf("canceled category = %s", got)
	}

	timedOut, cancelTimeout := context.WithTimeout(context.Background(), 0)
	defer cancelTimeout()
	if got := classifyRelayError(timedOut, errors.New("anything")); got != ErrorCategoryTimeout {
		t.Fatalf("timeout category = %s", got)
	}

	if got := classifyRelayError(context.Background(), errors.New("write: broken pipe")); got != ErrorCategoryClientCanceled {
		t.Fatalf("broken pipe category = %s", got)
	}
	if got := classifyRelayError(context.Background(), errors.New("i/o timeout")); got != ErrorCategoryTimeout {
		t.Fatalf("i/o timeout category = %s", got)
	}
}

func TestErrEmptyUpstreamResponseIsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("ctx: %w", errEmptyUpstreamResponse)
	if !errors.Is(wrapped, errEmptyUpstreamResponse) {
		t.Error("errors.Is should match wrapped sentinel")
	}
}

func TestCleanErrorMessage(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// 去除内部前缀 + 提取上游 message
		{`upstream status 502: {"error":{"message":"bad gateway"}}`, "bad gateway"},
		{"do request: dial tcp timeout", "dial tcp timeout"},
		{"stream: connection reset", "connection reset"},
		{errEmptyUpstreamResponse.Error(), "上游返回空响应"},
	}
	for _, c := range cases {
		if got := cleanErrorMessage(c.in); got != c.want {
			t.Errorf("cleanErrorMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFriendlyError_IncludesProviderName(t *testing.T) {
	info := &RelayInfo{OriginModel: "gpt-4o", Channel: &model.Channel{Name: "OpenAI-Main"}}
	// 致命错误应带供应商名
	msg := friendlyUpstreamError(info, http.StatusUnauthorized,
		fmt.Errorf("upstream status 401: %s", `{"error":{"message":"bad key"}}`))
	if !strings.Contains(msg, "OpenAI-Main") {
		t.Errorf("client error should include provider name: %q", msg)
	}
	// 耗尽错误应带最后供应商名
	ex := friendlyExhaustedError(info, http.StatusServiceUnavailable, "upstream status 503: overloaded")
	if !strings.Contains(ex, "OpenAI-Main") {
		t.Errorf("exhausted error should include provider name: %q", ex)
	}
}
