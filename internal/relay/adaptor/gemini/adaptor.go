package gemini

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type Adaptor struct {
	anthropicStream *protocol.AnthropicStreamEncoder
}

func NewAdaptor() *Adaptor {
	return &Adaptor{}
}

func (a *Adaptor) APIType() constant.APIType {
	return constant.APITypeGemini
}

func (a *Adaptor) GetRequestURL(baseURL string, mode constant.RelayMode) string {
	return a.GetRequestURLWithModel(baseURL, mode, "gemini-pro", false)
}

func (a *Adaptor) GetRequestURLWithModel(baseURL string, mode constant.RelayMode, model string, stream bool) string {
	baseURL = normalizeBaseURL(baseURL)
	model = normalizeModelPath(model)
	if model == "" {
		model = "gemini-pro"
	}

	methodSuffix := ":generateContent"
	if stream {
		methodSuffix = ":streamGenerateContent?alt=sse"
	}

	if strings.Contains(baseURL, "{model}") {
		return strings.ReplaceAll(baseURL, "{model}", model) + methodSuffixIfMissing(baseURL, methodSuffix)
	}
	if strings.Contains(baseURL, ":generateContent") || strings.Contains(baseURL, ":streamGenerateContent") {
		return baseURL
	}
	if strings.Contains(baseURL, "/models/") {
		return baseURL + methodSuffix
	}
	return baseURL + "/models/" + model + methodSuffix
}

func (a *Adaptor) SetupHeaders(headers http.Header, apiKey string, mode constant.RelayMode) {
	if apiKey != "" {
		headers.Set("x-goog-api-key", apiKey)
	}
	headers.Set("Content-Type", "application/json")
}

func (a *Adaptor) ConvertRequest(req []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return a.ConvertRequestWithMeta(req, mode, format, protocol.RequestMeta{})
}

func (a *Adaptor) ConvertRequestWithMeta(req []byte, mode constant.RelayMode, format constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
		return nil, fmt.Errorf("%s is not supported for gemini channels yet", mode)
	}

	switch format {
	case constant.RelayFormatGemini:
		return req, nil
	case constant.RelayFormatOpenAI:
		chatReq, err := protocol.OpenAIChatRequestToProtocol(req)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToGeminiGenerateContentRequest(chatReq)
	case constant.RelayFormatAnthropic:
		chatReq, err := protocol.AnthropicMessagesRequestToProtocol(req)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToGeminiGenerateContentRequest(chatReq)
	case constant.RelayFormatOpenAIResponses:
		return nil, fmt.Errorf("responses is not supported for gemini channels yet")
	default:
		return nil, fmt.Errorf("%s caller format is not supported for gemini channels yet", format)
	}
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
		return resp, nil
	}

	switch format {
	case constant.RelayFormatGemini:
		return resp, nil
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		chatResp, err := protocol.GeminiGenerateContentResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToOpenAIChatResponse(chatResp)
	case constant.RelayFormatAnthropic:
		chatResp, err := protocol.GeminiGenerateContentResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToAnthropicMessagesResponse(chatResp)
	default:
		return resp, nil
	}
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions || format == constant.RelayFormatGemini {
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
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" {
			continue
		}
		events, err := protocol.GeminiStreamEventsFromData(line)
		if err != nil {
			continue
		}
		for _, event := range events {
			encoded, err := a.encodeStreamEvent(event, format)
			if err != nil {
				return nil, err
			}
			result.Write(encoded)
			if format == constant.RelayFormatOpenAI && event.FinishReason != "" {
				doneBytes, err := protocol.ProtocolStreamEventToOpenAIData(protocol.StreamEvent{Done: true})
				if err != nil {
					return nil, err
				}
				result.Write(doneBytes)
			}
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
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		return protocol.ProtocolStreamEventToOpenAIData(event)
	default:
		return nil, nil
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

func normalizeModelPath(model string) string {
	model = strings.TrimSpace(strings.TrimPrefix(model, "/"))
	model = strings.TrimPrefix(model, "models/")
	return model
}

func methodSuffixIfMissing(url, suffix string) string {
	if strings.Contains(url, ":generateContent") || strings.Contains(url, ":streamGenerateContent") {
		return ""
	}
	return suffix
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
