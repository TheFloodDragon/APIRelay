package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestBuildRelayAttemptOpenAIPassthroughSkipsTransform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	body := []byte(`{"model":"gpt-test","stream":false,"messages":[{"role":"user","content":"hi"}]}`)
	reqCtx := &RequestContext{
		Gin:       ginCtx,
		RequestID: "passthrough-openai",
		StartTime: time.Now(),
		Mode:      constant.RelayModeChatCompletions,
		Format:    constant.RelayFormatOpenAI,
		Method:    http.MethodPost,
		RawBody:   body,
		Body:      body,
		Meta:      relayRequestMeta{Model: "gpt-test"},
	}
	candidate := relayCandidate{Channel: model.Channel{Type: "openai", BaseURL: "https://example.com/v1", APIKey: "sk-test"}, ResolvedModel: "gpt-test"}

	attempt, err := (&RelayController{}).buildRelayAttempt(reqCtx, candidate, false)
	if err != nil {
		t.Fatalf("buildRelayAttempt returned error: %v", err)
	}
	if attempt.NeedsTransform {
		t.Fatal("NeedsTransform = true, want false")
	}
	if !bytes.Equal(attempt.ConvertedBody, body) {
		t.Fatalf("ConvertedBody = %s, want original body", string(attempt.ConvertedBody))
	}
	if attempt.URL != "https://example.com/v1/chat/completions" {
		t.Fatalf("URL = %q", attempt.URL)
	}
}

func TestBuildRelayAttemptAnthropicToOpenAITransforms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	body := []byte(`{"model":"claude-test","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`)
	reqCtx := &RequestContext{
		Gin:       ginCtx,
		RequestID: "transform-anthropic-openai",
		StartTime: time.Now(),
		Mode:      constant.RelayModeMessages,
		Format:    constant.RelayFormatAnthropic,
		Method:    http.MethodPost,
		RawBody:   body,
		Body:      body,
		Meta:      relayRequestMeta{Model: "claude-test"},
	}
	candidate := relayCandidate{Channel: model.Channel{Type: "openai", BaseURL: "https://example.com/v1", APIKey: "sk-test"}, ResolvedModel: "gpt-test"}

	attempt, err := (&RelayController{}).buildRelayAttempt(reqCtx, candidate, false)
	if err != nil {
		t.Fatalf("buildRelayAttempt returned error: %v", err)
	}
	if !attempt.NeedsTransform {
		t.Fatal("NeedsTransform = false, want true")
	}
	if bytes.Equal(attempt.ConvertedBody, body) {
		t.Fatalf("ConvertedBody should differ from original body after protocol transform")
	}
}
