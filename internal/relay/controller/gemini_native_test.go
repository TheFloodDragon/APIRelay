package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestParseGeminiNativePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		rawQuery   string
		wantKind   geminiNativeRouteKind
		wantModel  string
		wantVer    string
		wantAct    string
		wantStream bool
	}{
		{
			name:      "v1beta generateContent",
			path:      "/v1beta/models/gemini-pro:generateContent",
			wantKind:  geminiNativeRouteGenerate,
			wantModel: "gemini-pro",
			wantVer:   "v1beta",
			wantAct:   "generateContent",
		},
		{
			name:       "gemini namespace streamGenerateContent",
			path:       "/gemini/v1beta/models/gemini-1.5-pro:streamGenerateContent",
			wantKind:   geminiNativeRouteGenerate,
			wantModel:  "gemini-1.5-pro",
			wantVer:    "v1beta",
			wantAct:    "streamGenerateContent",
			wantStream: true,
		},
		{
			name:       "generateContent with alt sse",
			path:       "/gemini/v1/models/gemini-pro:generateContent",
			rawQuery:   "alt=sse",
			wantKind:   geminiNativeRouteGenerate,
			wantModel:  "gemini-pro",
			wantVer:    "v1",
			wantAct:    "generateContent",
			wantStream: true,
		},
		{
			name:      "countTokens",
			path:      "/v1beta/models/gemini-pro:countTokens",
			wantKind:  geminiNativeRouteCountTokens,
			wantModel: "gemini-pro",
			wantVer:   "v1beta",
			wantAct:   "countTokens",
		},
		{
			name:      "model metadata",
			path:      "/v1beta/models/gemini-pro",
			wantKind:  geminiNativeRouteModel,
			wantModel: "gemini-pro",
			wantVer:   "v1beta",
		},
		{
			name:     "model list",
			path:     "/v1beta/models",
			wantKind: geminiNativeRouteModels,
			wantVer:  "v1beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGeminiNativePath(tt.path, tt.rawQuery)
			if err != nil {
				t.Fatalf("parseGeminiNativePath returned error: %v", err)
			}
			if got.Kind != tt.wantKind || got.Model != tt.wantModel || got.Version != tt.wantVer || got.Action != tt.wantAct || got.Stream != tt.wantStream {
				t.Fatalf("route = %#v, want kind=%s model=%q version=%q action=%q stream=%v", got, tt.wantKind, tt.wantModel, tt.wantVer, tt.wantAct, tt.wantStream)
			}
		})
	}
}

func TestParseGeminiNativePathRejectsUnsupportedAction(t *testing.T) {
	_, err := parseGeminiNativePath("/v1beta/models/gemini-pro:batchEmbedContents", "")
	if err == nil {
		t.Fatal("parseGeminiNativePath returned nil error, want unsupported action error")
	}
}

func TestBuildRelayAttemptGeminiCountTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:countTokens", bytes.NewReader([]byte(`{"contents":[{"parts":[{"text":"hello"}]}]}`)))

	reqCtx := &RequestContext{
		Gin:       ginCtx,
		RequestID: "test-request",
		StartTime: time.Now(),
		App:       constant.RelayAppGemini,
		Mode:      constant.RelayModeCountTokens,
		Format:    constant.RelayFormatGemini,
		Method:    http.MethodPost,
		Body:      []byte(`{"contents":[{"parts":[{"text":"hello"}]}]}`),
		Meta:      relayRequestMeta{Model: "gemini-pro"},
	}
	candidate := relayCandidate{Channel: model.Channel{
		Type:    "gemini",
		APIKey:  "AIza-upstream",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Config:  model.JSONMap{"auth_type": "api_key"},
	}, ResolvedModel: "gemini-pro"}

	attempt, err := (&RelayController{}).buildRelayAttempt(reqCtx, candidate, false)
	if err != nil {
		t.Fatalf("buildRelayAttempt returned error: %v", err)
	}
	if attempt.URL != "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:countTokens" {
		t.Fatalf("url = %q, want countTokens URL", attempt.URL)
	}
	if got := attempt.Headers.Get("x-goog-api-key"); got != "AIza-upstream" {
		t.Fatalf("x-goog-api-key = %q, want AIza-upstream", got)
	}
	if !bytes.Equal(attempt.ConvertedBody, reqCtx.Body) {
		t.Fatalf("converted body = %s, want original body", string(attempt.ConvertedBody))
	}
}

func TestGeminiNativeUnsupportedActionReturnsGeminiError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(writer)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:batchEmbedContents", nil)

	(&RelayController{}).GeminiNative(ginCtx)

	if writer.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", writer.Code, http.StatusBadRequest)
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(writer.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["error"]["status"] != "INVALID_ARGUMENT" {
		t.Fatalf("error status = %v, want INVALID_ARGUMENT", payload["error"]["status"])
	}
}
