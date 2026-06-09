package protocol

import (
	"fmt"
	"strings"
	"time"
)

// ChatRequest 是 APIRelay 内部用于承接 OpenAI / Anthropic / Gemini 文本聊天请求的最小公共表示。
type ChatRequest struct {
	Model       string
	System      string
	Messages    []ChatMessage
	Temperature *float64
	TopP        *float64
	Stream      bool
	Stop        []string
	MaxTokens   *int
	Tools       []map[string]interface{}
	ToolChoice  interface{}
}

// ChatMessage 表示通用文本消息。第一阶段仅保留基础角色与文本内容。
type ChatMessage struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []map[string]interface{}
}

// ChatResponse 是非流式文本聊天响应的最小公共表示。
type ChatResponse struct {
	ID           string
	Model        string
	Role         string
	Content      string
	ToolCalls    []map[string]interface{}
	FinishReason string
	Usage        *Usage
	Created      int64
}

// Usage 统一记录基础 token 用量。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamEvent 是流式文本增量的公共表示。
type StreamEvent struct {
	ID            string
	Model         string
	Role          string
	Content       string
	ToolCalls     []map[string]interface{}
	ToolCallID    string
	ToolName      string
	ToolArguments string
	FinishReason  string
	Index         int
	Start         bool
	Done          bool
}

// RequestMeta 是控制器已解析出的请求元信息，供 URL 带模型的入口（例如 Gemini）参与协议转换。
type RequestMeta struct {
	Model  string
	Stream bool
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func generatedID(prefix string) string {
	if prefix == "" {
		prefix = "msg"
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "model":
		return "assistant"
	case "system", "developer":
		return "system"
	case "tool":
		return "tool"
	default:
		return "user"
	}
}

func normalizeFinishReason(reason string) string {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "max_tokens", "length":
		return "length"
	case "content_filter", "safety", "recitation", "blocklist", "prohibited_content", "spii":
		return "content_filter"
	case "tool_use", "tool_calls", "function_call":
		return "tool_calls"
	case "stop_sequence", "end_turn", "stop", "":
		fallthrough
	default:
		return "stop"
	}
}

func textParts(parts []string, sep string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, sep)
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

func contentToText(content interface{}) string {
	switch value := content.(type) {
	case nil:
		return ""
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
	case []map[string]interface{}:
		parts := make([]string, 0, len(value))
		for _, part := range value {
			if text, ok := part["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
