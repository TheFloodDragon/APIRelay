package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
	"github.com/TheFloodDragon/APIRelay/internal/relay/transform"
)

const defaultBaseURL = "https://api.openai.com/v1"

type Adaptor struct {
	anthropicStream *protocol.AnthropicStreamEncoder
}

func NewAdaptor() *Adaptor {
	return &Adaptor{}
}

func (a *Adaptor) APIType() constant.APIType {
	return constant.APITypeOpenAI
}

func (a *Adaptor) Name() string {
	return constant.APITypeOpenAI.String()
}

func (a *Adaptor) ExtractBaseURL(channel *model.Channel) (string, error) {
	if channel == nil {
		return "", fmt.Errorf("channel is nil")
	}
	return channel.BaseURL, nil
}

func (a *Adaptor) ExtractAuth(channel *model.Channel) (string, map[string]interface{}) {
	if channel == nil {
		return "", nil
	}
	return channel.APIKey, channel.Config
}

func (a *Adaptor) BuildURL(baseURL string, mode constant.RelayMode, resolvedModel string, stream bool) string {
	return a.GetRequestURL(baseURL, mode)
}

func (a *Adaptor) GetAuthHeaders(apiKey string, config map[string]interface{}, mode constant.RelayMode, stream bool) (http.Header, error) {
	headers := http.Header{}
	a.SetupHeaders(headers, apiKey, mode)
	if stream {
		headers.Set("Accept", "text/event-stream")
	}
	return headers, nil
}

func (a *Adaptor) NeedsTransform(channel *model.Channel, callerFormat constant.RelayFormat) bool {
	return callerFormat != constant.RelayFormatOpenAI && callerFormat != constant.RelayFormatOpenAIResponses
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
	return a.ConvertRequestWithMeta(req, mode, format, protocol.RequestMeta{})
}

func (a *Adaptor) ConvertRequestWithMeta(req []byte, mode constant.RelayMode, format constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	return transform.RequestToOpenAI(req, mode, format, meta)
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return transform.ResponseFromOpenAI(resp, mode, format)
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() || format == constant.RelayFormatOpenAI || format == constant.RelayFormatOpenAIResponses {
		return chunk, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(chunk))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var result bytes.Buffer
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		events, err := protocol.OpenAIStreamEventsFromData(data)
		if err != nil {
			continue
		}
		for _, event := range events {
			encoded, err := a.encodeStreamEvent(event, format)
			if err != nil {
				return nil, err
			}
			result.Write(encoded)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func (a *Adaptor) encodeStreamEvent(event protocol.StreamEvent, format constant.RelayFormat) ([]byte, error) {
	switch format {
	case constant.RelayFormatAnthropic:
		if a.anthropicStream == nil {
			a.anthropicStream = protocol.NewAnthropicStreamEncoder()
		}
		return a.anthropicStream.Encode(event)
	case constant.RelayFormatGemini:
		return protocol.ProtocolStreamEventToGeminiData(event)
	default:
		return protocol.ProtocolStreamEventToOpenAIData(event)
	}
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
	case constant.RelayModeMessages, constant.RelayModeGeminiNative, constant.RelayModeChatCompletions:
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

func formatOpenAIErrorObject(errObj map[string]interface{}, raw []byte) string {
	parts := make([]string, 0, 6)
	if message, _ := errObj["message"].(string); message != "" {
		parts = append(parts, message)
	}
	if typ, _ := errObj["type"].(string); typ != "" {
		parts = append(parts, "type="+typ)
	}
	if code, _ := errObj["code"].(string); code != "" {
		parts = append(parts, "code="+code)
	} else if code := errObj["code"]; code != nil {
		parts = append(parts, fmt.Sprintf("code=%v", code))
	}
	if param, _ := errObj["param"].(string); param != "" {
		parts = append(parts, "param="+param)
	}
	if len(parts) == 0 {
		return string(raw)
	}
	if isGenericOpenAIWrapperError(errObj) {
		parts = append(parts, "raw="+string(raw))
	}
	return strings.Join(parts, "; ")
}

func isGenericOpenAIWrapperError(errObj map[string]interface{}) bool {
	message, _ := errObj["message"].(string)
	typ, _ := errObj["type"].(string)
	code, _ := errObj["code"].(string)
	return message == "openai_error" || typ == "bad_response_status_code" || code == "bad_response_status_code"
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
			return formatOpenAIErrorObject(errObj, resp)
		case string:
			return errObj
		}
	}

	if message, ok := payload["message"].(string); ok && message != "" {
		return message
	}

	return string(resp)
}
