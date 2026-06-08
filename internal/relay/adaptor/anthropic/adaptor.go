package anthropic

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

const (
	defaultBaseURL          = "https://api.anthropic.com/v1"
	defaultAnthropicVersion = "2023-06-01"
)

type Adaptor struct{}

func NewAdaptor() *Adaptor {
	return &Adaptor{}
}

func (a *Adaptor) APIType() constant.APIType {
	return constant.APITypeAnthropic
}

func (a *Adaptor) GetRequestURL(baseURL string, mode constant.RelayMode) string {
	baseURL = normalizeBaseURL(baseURL)
	if strings.HasSuffix(baseURL, "/messages") {
		return baseURL
	}
	return baseURL + "/messages"
}

func (a *Adaptor) SetupHeaders(headers http.Header, apiKey string, mode constant.RelayMode) {
	if apiKey != "" {
		headers.Set("x-api-key", apiKey)
	}
	headers.Set("anthropic-version", defaultAnthropicVersion)
	headers.Set("Content-Type", "application/json")
}

func (a *Adaptor) ConvertRequest(req []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	return a.ConvertRequestWithMeta(req, mode, format, protocol.RequestMeta{})
}

func (a *Adaptor) ConvertRequestWithMeta(req []byte, mode constant.RelayMode, format constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error) {
	if !mode.IsChatLike() {
		return nil, fmt.Errorf("%s is not supported for anthropic channels yet", mode)
	}

	switch format {
	case constant.RelayFormatAnthropic:
		return req, nil
	case constant.RelayFormatOpenAI:
		chatReq, err := protocol.OpenAIChatRequestToProtocol(req)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToAnthropicMessagesRequest(chatReq)
	case constant.RelayFormatGemini:
		chatReq, err := protocol.GeminiGenerateContentRequestToProtocol(req, meta.Model, meta.Stream)
		if err != nil {
			return nil, err
		}
		if meta.Model != "" {
			chatReq.Model = meta.Model
		}
		return protocol.ProtocolToAnthropicMessagesRequest(chatReq)
	case constant.RelayFormatOpenAIResponses:
		return nil, fmt.Errorf("responses is not supported for anthropic channels yet")
	default:
		return nil, fmt.Errorf("%s caller format is not supported for anthropic channels yet", format)
	}
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() {
		return resp, nil
	}

	switch format {
	case constant.RelayFormatAnthropic:
		return resp, nil
	case constant.RelayFormatOpenAI, constant.RelayFormatOpenAIResponses:
		chatResp, err := protocol.AnthropicMessagesResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToOpenAIChatResponse(chatResp)
	case constant.RelayFormatGemini:
		chatResp, err := protocol.AnthropicMessagesResponseToProtocol(resp)
		if err != nil {
			return nil, err
		}
		return protocol.ProtocolToGeminiGenerateContentResponse(chatResp)
	default:
		return resp, nil
	}
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if !mode.IsChatLike() || format == constant.RelayFormatAnthropic {
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
		events, err := protocol.AnthropicStreamEventsFromData(data)
		if err != nil {
			continue
		}
		for _, event := range events {
			encoded, err := encodeStreamEvent(event, format)
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

func encodeStreamEvent(event protocol.StreamEvent, format constant.RelayFormat) ([]byte, error) {
	switch format {
	case constant.RelayFormatGemini:
		return protocol.ProtocolStreamEventToGeminiData(event)
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
