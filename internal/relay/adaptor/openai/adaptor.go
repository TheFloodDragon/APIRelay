package openai

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
	if mode == constant.RelayModeCountTokens {
		return nil, fmt.Errorf("%s is not supported for openai channels yet", mode)
	}
	switch format {
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		return req, nil
	case constant.RelayFormatAnthropic:
		if !mode.IsChatLike() {
			return nil, fmt.Errorf("%s is not supported for anthropic caller format on openai channels yet", mode)
		}
		chatReq, err := protocol.AnthropicMessagesRequestToProtocol(req)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToOpenAIChatRequest(chatReq)
	case constant.RelayFormatGemini:
		if !mode.IsChatLike() {
			return nil, fmt.Errorf("%s is not supported for gemini caller format on openai channels yet", mode)
		}
		chatReq, err := protocol.GeminiGenerateContentRequestToProtocol(req, meta.Model, meta.Stream)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToOpenAIChatRequest(chatReq)
	default:
		return req, nil
	}
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() {
		return resp, nil
	}

	switch format {
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		return resp, nil
	case constant.RelayFormatAnthropic:
		chatResp, err := protocol.OpenAIChatResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToAnthropicMessagesResponse(chatResp)
	case constant.RelayFormatGemini:
		chatResp, err := protocol.OpenAIChatResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToGeminiGenerateContentResponse(chatResp)
	default:
		return resp, nil
	}
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
