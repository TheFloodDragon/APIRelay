package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

type anthropicMessagesRequest struct {
	Model         string             `json:"model"`
	Messages      []anthropicMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	System        interface{}        `json:"system,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
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
	Text string `json:"text,omitempty"`
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

// AnthropicMessagesRequestToProtocol 将 Anthropic Messages 请求转为通用文本聊天请求。
func AnthropicMessagesRequestToProtocol(body []byte) (*ChatRequest, error) {
	var req anthropicMessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 请求失败: %w", err)
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		System:      contentToText(req.System),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
	}
	if req.MaxTokens > 0 {
		chatReq.MaxTokens = &req.MaxTokens
	}
	for _, message := range req.Messages {
		role := normalizeRole(message.Role)
		if role == "system" {
			role = "user"
		}
		text := contentToText(message.Content)
		if text == "" {
			continue
		}
		chatReq.Messages = append(chatReq.Messages, ChatMessage{Role: role, Content: text})
	}
	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("缺少可转发的 messages")
	}
	return chatReq, nil
}

// ProtocolToAnthropicMessagesRequest 将通用文本聊天请求转为 Anthropic Messages 请求。
func ProtocolToAnthropicMessagesRequest(req *ChatRequest) ([]byte, error) {
	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	anthropicReq := anthropicMessagesRequest{
		Model:         req.Model,
		MaxTokens:     maxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		Stream:        req.Stream,
		StopSequences: req.Stop,
	}
	if req.System != "" {
		anthropicReq.System = req.System
	}
	for _, message := range req.Messages {
		role := normalizeRole(message.Role)
		if role == "system" {
			role = "user"
		}
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{Role: role, Content: message.Content})
	}
	if len(anthropicReq.Messages) == 0 && req.System != "" {
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{Role: "user", Content: req.System})
		anthropicReq.System = nil
	}
	return json.Marshal(anthropicReq)
}

// AnthropicMessagesResponseToProtocol 将 Anthropic Messages 响应转为通用响应。
func AnthropicMessagesResponseToProtocol(body []byte) (*ChatResponse, error) {
	var resp anthropicMessagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}

	parts := make([]string, 0, len(resp.Content))
	for _, item := range resp.Content {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
	}
	chatResp := &ChatResponse{
		ID:           firstNonEmpty(resp.ID, generatedID("msg")),
		Model:        resp.Model,
		Role:         firstNonEmpty(resp.Role, "assistant"),
		Content:      strings.Join(parts, ""),
		FinishReason: normalizeFinishReason(resp.StopReason),
		Created:      nowUnix(),
	}
	if resp.Usage != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	return chatResp, nil
}

// ProtocolToAnthropicMessagesResponse 将通用响应转为 Anthropic Messages 响应。
func ProtocolToAnthropicMessagesResponse(resp *ChatResponse) ([]byte, error) {
	anthropicResp := anthropicMessagesResponse{
		ID:         firstNonEmpty(resp.ID, generatedID("msg")),
		Type:       "message",
		Role:       firstNonEmpty(resp.Role, "assistant"),
		Model:      resp.Model,
		StopReason: finishReasonToAnthropic(resp.FinishReason),
		Content: []anthropicContent{
			{Type: "text", Text: resp.Content},
		},
	}
	if anthropicResp.Role == "model" {
		anthropicResp.Role = "assistant"
	}
	if resp.Usage != nil {
		anthropicResp.Usage = &anthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return json.Marshal(anthropicResp)
}

// AnthropicStreamEventsFromData 将单条 Anthropic SSE data 负载转为通用流事件。
func AnthropicStreamEventsFromData(data string) ([]StreamEvent, error) {
	if data == "[DONE]" {
		return []StreamEvent{{Done: true}}, nil
	}
	var chunk anthropicStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, err
	}

	switch chunk.Type {
	case "message_start":
		event := StreamEvent{Start: true, Role: "assistant"}
		if chunk.Message != nil {
			event.ID = chunk.Message.ID
			event.Model = chunk.Message.Model
		}
		return []StreamEvent{event}, nil
	case "content_block_start":
		if chunk.ContentBlock != nil && chunk.ContentBlock.Text != "" {
			return []StreamEvent{{Content: chunk.ContentBlock.Text, Index: chunk.Index}}, nil
		}
	case "content_block_delta":
		if chunk.Delta != nil && chunk.Delta.Text != "" {
			return []StreamEvent{{Content: chunk.Delta.Text, Index: chunk.Index}}, nil
		}
	case "message_delta":
		if chunk.Delta != nil && chunk.Delta.StopReason != "" {
			return []StreamEvent{{FinishReason: normalizeFinishReason(chunk.Delta.StopReason)}}, nil
		}
	case "message_stop":
		return []StreamEvent{{Done: true}}, nil
	}
	return nil, nil
}

func finishReasonToAnthropic(reason string) string {
	switch normalizeFinishReason(reason) {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}
