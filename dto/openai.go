package dto

import (
	"bytes"
	"encoding/json"
)

// StopSequences 兼容 OpenAI stop 字段的两种形态：单个字符串 "x" 或字符串数组 ["x","y"]。
// 反序列化时统一归一为 []string，序列化时原样输出为数组（或省略）。
type StopSequences []string

// UnmarshalJSON 兼容字符串与字符串数组两种输入。
func (s *StopSequences) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*s = nil
		return nil
	}
	// 数组形态
	if data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	// 单字符串形态
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*s = StopSequences{single}
	return nil
}

// MarshalJSON 原样输出为数组（nil/空时输出 null，配合 omitempty 会被省略）。
func (s StopSequences) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("null"), nil
	}
	return json.Marshal([]string(s))
}

// ============================================================================
// OpenAI Chat Completions 协议结构
// ============================================================================

// OpenAIChatRequest 对应 /v1/chat/completions 请求。
type OpenAIChatRequest struct {
	Model             string                `json:"model"`
	Messages          []OpenAIMessage       `json:"messages"`
	MaxTokens         *int                  `json:"max_tokens,omitempty"`
	Temperature       *float64              `json:"temperature,omitempty"`
	TopP              *float64              `json:"top_p,omitempty"`
	Stream            bool                  `json:"stream,omitempty"`
	Stop              StopSequences         `json:"stop,omitempty"`
	Tools             []OpenAITool          `json:"tools,omitempty"`
	ToolChoice        json.RawMessage       `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool                 `json:"parallel_tool_calls,omitempty"`
	ResponseFormat    *OpenAIResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort   string                `json:"reasoning_effort,omitempty"`
	TopK              *int                  `json:"top_k,omitempty"`
	// StreamOptions 用于在流式中请求 usage
	StreamOptions *OpenAIStreamOptions `json:"stream_options,omitempty"`
}

type OpenAIResponseFormat struct {
	Type       string            `json:"type"`
	JSONSchema *OpenAIJSONSchema `json:"json_schema,omitempty"`
}

type OpenAIJSONSchema struct {
	Name   string          `json:"name,omitempty"`
	Strict bool            `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// OpenAIMessage role + content(可为 string 或多模态数组)。
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

type OpenAIToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIToolCallFunc `json:"function"`
	Index    *int               `json:"index,omitempty"`
}

type OpenAIToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ---- 响应 ----

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

type OpenAIChoice struct {
	Index        int            `json:"index"`
	Message      *OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIDelta   `json:"delta,omitempty"`
	FinishReason *string        `json:"finish_reason"`
}

type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens            int                           `json:"prompt_tokens"`
	CompletionTokens        int                           `json:"completion_tokens"`
	TotalTokens             int                           `json:"total_tokens"`
	PromptTokensDetails     *OpenAIPromptTokenDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *OpenAICompletionTokenDetails `json:"completion_tokens_details,omitempty"`
}

type OpenAIPromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type OpenAICompletionTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// OpenAIErrorResponse 标准 OpenAI 错误体。
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
