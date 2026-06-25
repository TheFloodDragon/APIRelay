package dto

import "encoding/json"

// ============================================================================
// Anthropic Messages 协议结构（/v1/messages）
// ============================================================================

// AnthropicRequest 对应 Anthropic Messages 请求。
type AnthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	System        json.RawMessage    `json:"system,omitempty"` // string 或 content block 数组
	MaxTokens     int                `json:"max_tokens"`
	Stream        bool               `json:"stream,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
}

// AnthropicMessage role + content（string 或 content block 数组）。
type AnthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// AnthropicContentBlock 内容块（text / image / tool_use / tool_result）。
type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`

	// image
	Source *AnthropicImageSource `json:"source,omitempty"`
}

type AnthropicImageSource struct {
	Type      string `json:"type"`       // base64 | url
	MediaType string `json:"media_type"` // image/png ...
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ---- 响应 ----

type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"` // "message"
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []AnthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---- 流式事件 ----

// AnthropicStreamEvent 是 SSE 中的一个事件（event: <type> + data: <json>）。
type AnthropicStreamEvent struct {
	Type string `json:"type"`

	// message_start
	Message *AnthropicResponse `json:"message,omitempty"`

	// content_block_start / content_block_delta
	Index        int                    `json:"index,omitempty"`
	ContentBlock *AnthropicContentBlock `json:"content_block,omitempty"`
	Delta        *AnthropicStreamDelta  `json:"delta,omitempty"`

	// message_delta
	Usage *AnthropicUsage `json:"usage,omitempty"`
}

type AnthropicStreamDelta struct {
	Type        string `json:"type,omitempty"` // text_delta | input_json_delta
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	// message_delta 的 stop_reason
	StopReason string `json:"stop_reason,omitempty"`
}

// AnthropicErrorResponse 标准 Anthropic 错误体。
type AnthropicErrorResponse struct {
	Type  string         `json:"type"` // "error"
	Error AnthropicError `json:"error"`
}

type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
