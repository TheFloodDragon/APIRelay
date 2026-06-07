package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
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
	if mode == constant.RelayModeResponses {
		return nil, fmt.Errorf("responses is not supported for anthropic/gemini channels yet")
	}
	if mode != constant.RelayModeChatCompletions {
		return nil, fmt.Errorf("%s is not supported for anthropic channels yet", mode)
	}

	var openaiReq openAIChatRequest
	if err := json.Unmarshal(req, &openaiReq); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}

	anthropicReq := anthropicMessagesRequest{
		Model:       openaiReq.Model,
		Temperature: openaiReq.Temperature,
		TopP:        openaiReq.TopP,
		Stream:      openaiReq.Stream,
	}

	if openaiReq.MaxTokens != nil {
		anthropicReq.MaxTokens = *openaiReq.MaxTokens
	} else if openaiReq.MaxCompletionTokens != nil {
		anthropicReq.MaxTokens = *openaiReq.MaxCompletionTokens
	} else {
		anthropicReq.MaxTokens = 4096
	}

	anthropicReq.StopSequences = stringList(openaiReq.Stop)

	var systemMessages []string
	anthropicReq.Messages = make([]anthropicMessage, 0, len(openaiReq.Messages))
	for _, message := range openaiReq.Messages {
		role := strings.ToLower(message.Role)
		text := contentToText(message.Content)
		if text == "" {
			continue
		}

		switch role {
		case "system", "developer":
			systemMessages = append(systemMessages, text)
		case "assistant":
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{Role: "assistant", Content: text})
		default:
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{Role: "user", Content: text})
		}
	}
	anthropicReq.System = strings.Join(systemMessages, "\n\n")

	if len(anthropicReq.Messages) == 0 {
		return nil, fmt.Errorf("缺少可转发到 Anthropic 的 messages")
	}

	return json.Marshal(anthropicReq)
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
		return resp, nil
	}

	var anthropicResp anthropicMessagesResponse
	if err := json.Unmarshal(resp, &anthropicResp); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}

	var content strings.Builder
	for _, item := range anthropicResp.Content {
		if item.Type == "text" {
			content.WriteString(item.Text)
		}
	}

	id := anthropicResp.ID
	if id == "" {
		id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}

	finishReason := convertStopReason(anthropicResp.StopReason)
	openaiResp := openAIChatResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []openAIChatChoice{
			{
				Index: 0,
				Message: &openAIChatMessage{
					Role:    "assistant",
					Content: content.String(),
				},
				FinishReason: &finishReason,
			},
		},
	}

	if anthropicResp.Usage != nil {
		openaiResp.Usage = &openAIUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		}
	}

	return json.Marshal(openaiResp)
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
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
		if data == "[DONE]" {
			result.WriteString("data: [DONE]\n\n")
			continue
		}

		var anthropicChunk anthropicStreamChunk
		if err := json.Unmarshal([]byte(data), &anthropicChunk); err != nil {
			continue
		}

		for _, openaiChunk := range convertStreamChunkToOpenAI(&anthropicChunk) {
			if done, _ := openaiChunk["__done"].(bool); done {
				result.WriteString("data: [DONE]\n\n")
				continue
			}
			chunkBytes, err := json.Marshal(openaiChunk)
			if err != nil {
				continue
			}
			result.WriteString("data: ")
			result.Write(chunkBytes)
			result.WriteString("\n\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func (a *Adaptor) ErrorMessage(resp []byte) string {
	return parseErrorMessage(resp)
}

type openAIChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Temperature         *float64            `json:"temperature,omitempty"`
	TopP                *float64            `json:"top_p,omitempty"`
	Stream              bool                `json:"stream,omitempty"`
	Stop                interface{}         `json:"stop,omitempty"`
	MaxTokens           *int                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                `json:"max_completion_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type openAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []openAIChatChoice `json:"choices"`
	Usage   *openAIUsage       `json:"usage,omitempty"`
}

type openAIChatChoice struct {
	Index        int                `json:"index"`
	Message      *openAIChatMessage `json:"message,omitempty"`
	Delta        map[string]string  `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type anthropicMessagesRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	System        string             `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessagesResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason,omitempty"`
	Usage      *anthropicUsage    `json:"usage,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamChunk struct {
	Type         string                     `json:"type"`
	Index        int                        `json:"index,omitempty"`
	Delta        *anthropicDelta            `json:"delta,omitempty"`
	ContentBlock *anthropicContent          `json:"content_block,omitempty"`
	Message      *anthropicMessagesResponse `json:"message,omitempty"`
}

type anthropicDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

var doneSentinel = map[string]interface{}{"__done": true}

func convertStreamChunkToOpenAI(chunk *anthropicStreamChunk) []map[string]interface{} {
	switch chunk.Type {
	case "message_start":
		modelName := ""
		id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
		if chunk.Message != nil {
			modelName = chunk.Message.Model
			if chunk.Message.ID != "" {
				id = chunk.Message.ID
			}
		}
		return []map[string]interface{}{openAIStreamChunk(id, modelName, map[string]string{"role": "assistant"}, nil, 0)}
	case "content_block_start":
		if chunk.ContentBlock != nil && chunk.ContentBlock.Text != "" {
			return []map[string]interface{}{openAIStreamChunk("", "", map[string]string{"content": chunk.ContentBlock.Text}, nil, chunk.Index)}
		}
	case "content_block_delta":
		if chunk.Delta != nil && chunk.Delta.Text != "" {
			return []map[string]interface{}{openAIStreamChunk("", "", map[string]string{"content": chunk.Delta.Text}, nil, chunk.Index)}
		}
	case "message_delta":
		if chunk.Delta != nil && chunk.Delta.StopReason != "" {
			finishReason := convertStopReason(chunk.Delta.StopReason)
			return []map[string]interface{}{openAIStreamChunk("", "", map[string]string{}, &finishReason, 0)}
		}
	case "message_stop":
		return []map[string]interface{}{doneSentinel}
	}
	return nil
}

func openAIStreamChunk(id, model string, delta map[string]string, finishReason *string, index int) map[string]interface{} {
	if id == "" {
		id = fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	return map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index":         index,
				"delta":         delta,
				"finish_reason": finishReason,
			},
		},
	}
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func contentToText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok && text != "" {
				parts = append(parts, text)
				continue
			}
			if text, ok := part["input_text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func stringList(value interface{}) []string {
	switch stop := value.(type) {
	case nil:
		return nil
	case string:
		if stop == "" {
			return nil
		}
		return []string{stop}
	case []string:
		return stop
	case []interface{}:
		items := make([]string, 0, len(stop))
		for _, item := range stop {
			if text, ok := item.(string); ok && text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func convertStopReason(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "stop_sequence", "end_turn", "":
		fallthrough
	default:
		return "stop"
	}
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

