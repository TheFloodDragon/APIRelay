package openai

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

const defaultBaseURL = "https://api.openai.com/v1"

type Adaptor struct{}

func NewAdaptor() *Adaptor {
	return &Adaptor{}
}

func (a *Adaptor) APIType() constant.APIType {
	return constant.APITypeOpenAI
}

func (a *Adaptor) GetRequestURL(baseURL string, mode constant.RelayMode) string {
	return joinURL(normalizeBaseURL(baseURL), modePath(mode))
}

func (a *Adaptor) SetupHeaders(headers http.Header, apiKey string, mode constant.RelayMode) {
	if apiKey != "" {
		headers.Set("Authorization", "Bearer "+apiKey)
	}
	headers.Set("Content-Type", "application/json")
}

func (a *Adaptor) ConvertRequest(req []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return req, nil
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return resp, nil
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return chunk, nil
}

func (a *Adaptor) ErrorMessage(resp []byte) string {
	return parseErrorMessage(resp)
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func modePath(mode constant.RelayMode) string {
	switch mode {
	case constant.RelayModeResponses:
		return "/responses"
	case constant.RelayModeCompletions:
		return "/completions"
	case constant.RelayModeEmbeddings:
		return "/embeddings"
	case constant.RelayModeChatCompletions:
		fallthrough
	default:
		return "/chat/completions"
	}
}

func joinURL(baseURL, path string) string {
	if strings.HasSuffix(baseURL, path) {
		return baseURL
	}
	return baseURL + path
}

func parseErrorMessage(resp []byte) string {
	if len(resp) == 0 {
		return ""
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return string(resp)
	}

	if errorValue, ok := payload["error"]; ok {
		switch errObj := errorValue.(type) {
		case map[string]interface{}:
			if message, ok := errObj["message"].(string); ok && message != "" {
				return message
			}
		case string:
			return errObj
		}
	}

	if message, ok := payload["message"].(string); ok && message != "" {
		return message
	}

	return string(resp)
}
