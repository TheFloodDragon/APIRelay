package dto

import "encoding/json"

// ============================================================================
// 内部规范中枢 (Intermediate Representation, IR)
//
// 任意对外协议先解析为 UnifiedRequest，再由 Adaptor 转换为上游协议；
// 上游响应/流式事件先归一为 UnifiedResponse/UnifiedStreamChunk，再由 handler
// 按对外协议序列化输出。新增协议 = 1 个入站解析 + 1 个出站序列化，避免 N² 互转。
// ============================================================================

// UnifiedRole 统一角色。
type UnifiedRole string

const (
	RoleSystem    UnifiedRole = "system"
	RoleUser      UnifiedRole = "user"
	RoleAssistant UnifiedRole = "assistant"
	RoleTool      UnifiedRole = "tool"
)

// UnifiedContentPart 统一的多模态内容块。
type UnifiedContentPart struct {
	Type string `json:"type"` // text | image_url | ...
	Text string `json:"text,omitempty"`
	// ImageURL 用于图像输入
	ImageURL string `json:"image_url,omitempty"`
}

// UnifiedMessage 统一消息。
type UnifiedMessage struct {
	Role    UnifiedRole          `json:"role"`
	Content string               `json:"content,omitempty"`
	Parts   []UnifiedContentPart `json:"parts,omitempty"`
	// Name 工具消息的工具名 / 函数名
	Name string `json:"name,omitempty"`
	// ToolCallID 工具结果对应的调用 ID
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolCalls 助手发起的工具调用
	ToolCalls []UnifiedToolCall `json:"tool_calls,omitempty"`
}

// UnifiedToolCall 统一工具调用。
type UnifiedToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
	// Index 标识该工具调用在一次响应中的稳定序号（流式分片归并所需）。
	// nil 表示未知（非流式或上游未提供）。跨协议流式时用于区分并正确重组多个并行工具调用。
	Index *int `json:"index,omitempty"`
}

// UnifiedTool 统一工具定义。
type UnifiedTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// UnifiedRequest 统一请求（IR）。
type UnifiedRequest struct {
	Model       string           `json:"model"`
	System      string           `json:"system,omitempty"`
	Messages    []UnifiedMessage `json:"messages"`
	Tools       []UnifiedTool    `json:"tools,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	Stream      bool             `json:"stream"`
	Stop        []string         `json:"stop,omitempty"`

	// SourceEndpoint 记录对外协议来源，便于日志与差异化处理
	SourceEndpoint string `json:"-"`
	// Raw 保存原始请求体，用于 passthrough（同协议直转可不重新序列化）
	Raw json.RawMessage `json:"-"`
}

// Usage 统一用量。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// UnifiedResponse 统一的非流式响应。
type UnifiedResponse struct {
	ID           string            `json:"id"`
	Model        string            `json:"model"`
	Content      string            `json:"content"`
	ToolCalls    []UnifiedToolCall `json:"tool_calls,omitempty"`
	FinishReason string            `json:"finish_reason"`
	Usage        Usage             `json:"usage"`
}

// UnifiedStreamChunk 统一的流式增量。
type UnifiedStreamChunk struct {
	DeltaText    string            `json:"delta_text,omitempty"`
	ToolCalls    []UnifiedToolCall `json:"tool_calls,omitempty"`
	FinishReason string            `json:"finish_reason,omitempty"`
	Usage        *Usage            `json:"usage,omitempty"`
	// Done 标记流结束
	Done bool `json:"-"`
	// Raw 原始 SSE 行（用于零改写透传模式）
	// 当 IsRaw 为 true 时，DeltaText/ToolCalls 等字段会被忽略，直接写出原始行。
	// Raw 可以为空字符串（表示 SSE 的空行/事件边界），因此必须配合 IsRaw 判断。
	Raw string `json:"-"`
	// IsRaw 标记该 chunk 为零改写透传的原始 SSE 行。
	// 即使 Raw == ""（空行）也应原样写出，以保留 SSE 事件边界。
	IsRaw bool `json:"-"`
}
