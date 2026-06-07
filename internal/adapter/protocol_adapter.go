package adapter

import (
	"bytes"
	"encoding/json"
	"io"
)

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	// ConvertRequest 将 OpenAI 格式请求转换为目标协议格式
	ConvertRequest(openaiReq interface{}) (interface{}, error)

	// ConvertResponse 将目标协议格式响应转换为 OpenAI 格式
	ConvertResponse(targetResp io.Reader) ([]byte, error)

	// ConvertStreamChunk 将目标协议的流式 chunk 转换为 OpenAI SSE 格式
	ConvertStreamChunk(targetChunk []byte) ([]byte, error)

	// NeedsConversion 判断该渠道类型是否需要协议转换
	NeedsConversion() bool
}

// GetAdapter 根据渠道类型获取对应的协议适配器
func GetAdapter(channelType string) ProtocolAdapter {
	switch channelType {
	case "anthropic":
		return &AnthropicAdapter{}
	case "gemini":
		return &GeminiAdapter{}
	case "openai", "openai_compatible", "deepseek", "codex":
		return &PassthroughAdapter{}
	default:
		return &PassthroughAdapter{}
	}
}

// PassthroughAdapter 直通适配器（用于 OpenAI 兼容接口）
type PassthroughAdapter struct{}

func (a *PassthroughAdapter) ConvertRequest(openaiReq interface{}) (interface{}, error) {
	return openaiReq, nil
}

func (a *PassthroughAdapter) ConvertResponse(targetResp io.Reader) ([]byte, error) {
	return io.ReadAll(targetResp)
}

func (a *PassthroughAdapter) ConvertStreamChunk(targetChunk []byte) ([]byte, error) {
	return targetChunk, nil
}

func (a *PassthroughAdapter) NeedsConversion() bool {
	return false
}

// OpenAIChatRequest OpenAI 聊天请求格式
type OpenAIChatRequest struct {
	Model       string                   `json:"model"`
	Messages    []OpenAIChatMessage      `json:"messages"`
	Temperature *float64                 `json:"temperature,omitempty"`
	TopP        *float64                 `json:"top_p,omitempty"`
	N           *int                     `json:"n,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Stop        interface{}              `json:"stop,omitempty"`
	MaxTokens   *int                     `json:"max_tokens,omitempty"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// OpenAIChatResponse OpenAI 聊天响应格式
type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenAIChatChoice `json:"choices"`
	Usage   *OpenAIUsage       `json:"usage,omitempty"`
}

type OpenAIChatChoice struct {
	Index        int                `json:"index"`
	Message      *OpenAIChatMessage `json:"message,omitempty"`
	Delta        *OpenAIChatMessage `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DecodeJSON 解码 JSON 到目标结构
func DecodeJSON(data []byte, v interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	return decoder.Decode(v)
}
