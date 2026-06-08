package gemini

import (
	"net/http"
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

func TestGetRequestURLWithModel(t *testing.T) {
	adaptor := NewAdaptor()
	tests := []struct {
		name    string
		baseURL string
		mode    constant.RelayMode
		model   string
		stream  bool
		want    string
	}{
		{
			name:    "generateContent appends model and action",
			baseURL: "https://generativelanguage.googleapis.com/v1beta",
			mode:    constant.RelayModeGeminiNative,
			model:   "models/gemini-pro",
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		},
		{
			name:    "streamGenerateContent includes alt sse",
			baseURL: "https://generativelanguage.googleapis.com/v1beta",
			mode:    constant.RelayModeGeminiNative,
			model:   "gemini-pro",
			stream:  true,
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:streamGenerateContent?alt=sse",
		},
		{
			name:    "countTokens appends countTokens action",
			baseURL: "https://generativelanguage.googleapis.com/v1beta",
			mode:    constant.RelayModeCountTokens,
			model:   "gemini-pro",
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:countTokens",
		},
		{
			name:    "placeholder keeps explicit path",
			baseURL: "https://example.test/v1beta/models/{model}",
			mode:    constant.RelayModeCountTokens,
			model:   "gemini-pro",
			want:    "https://example.test/v1beta/models/gemini-pro:countTokens",
		},
		{
			name:    "explicit stream action gets alt sse",
			baseURL: "https://example.test/v1beta/models/gemini-pro:streamGenerateContent",
			mode:    constant.RelayModeGeminiNative,
			model:   "ignored",
			stream:  true,
			want:    "https://example.test/v1beta/models/gemini-pro:streamGenerateContent?alt=sse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adaptor.GetRequestURLWithModel(tt.baseURL, tt.mode, tt.model, tt.stream)
			if got != tt.want {
				t.Fatalf("url = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetupHeadersWithConfigSupportsAPIKeyAndBearer(t *testing.T) {
	adaptor := NewAdaptor()

	apiKeyHeaders := http.Header{}
	adaptor.SetupHeadersWithConfig(apiKeyHeaders, "AIza-test", constant.RelayModeGeminiNative, nil)
	if got := apiKeyHeaders.Get("x-goog-api-key"); got != "AIza-test" {
		t.Fatalf("x-goog-api-key = %q, want AIza-test", got)
	}
	if got := apiKeyHeaders.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}

	bearerHeaders := http.Header{}
	adaptor.SetupHeadersWithConfig(bearerHeaders, "ya29.test", constant.RelayModeGeminiNative, map[string]interface{}{"auth_type": "oauth"})
	if got := bearerHeaders.Get("Authorization"); got != "Bearer ya29.test" {
		t.Fatalf("Authorization = %q, want Bearer ya29.test", got)
	}
	if got := bearerHeaders.Get("x-goog-api-key"); got != "" {
		t.Fatalf("x-goog-api-key = %q, want empty", got)
	}

	prefixedHeaders := http.Header{}
	adaptor.SetupHeadersWithConfig(prefixedHeaders, "Bearer existing", constant.RelayModeGeminiNative, nil)
	if got := prefixedHeaders.Get("Authorization"); got != "Bearer existing" {
		t.Fatalf("Authorization = %q, want Bearer existing", got)
	}
}
