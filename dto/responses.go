package dto

import "encoding/json"

// ============================================================================
// OpenAI Responses 协议结构（/v1/responses）
//
// Responses 是 OpenAI 较新的协议，输入为 input（string 或 item 数组），
// 输出为 output（item 数组）。这里实现满足文本/工具/流式互转的子集。
// ============================================================================

// ResponsesRequest 对应 /v1/responses 请求。
type ResponsesRequest struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input"`                  // string 或 []ResponsesInputItem
	Instructions    string          `json:"instructions,omitempty"` // 等价于 system
	MaxOutputTokens *int            `json:"max_output_tokens,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	Tools           []ResponsesTool `json:"tools,omitempty"`
}

// ResponsesInputItem 输入项（message / function_call / function_call_output）。
type ResponsesInputItem struct {
	Type    string          `json:"type,omitempty"` // message | function_call | function_call_output
	Role    string          `json:"role,omitempty"`
	Content json.RawMessage `json:"content,omitempty"` // string 或 []ResponsesContentPart

	// function_call
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// function_call_output
	Output string `json:"output,omitempty"`
}

// ResponsesContentPart 输入/输出内容部分。
type ResponsesContentPart struct {
	Type     string `json:"type"` // input_text | output_text | input_image
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type ResponsesTool struct {
	Type        string          `json:"type"` // function
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ---- 响应 ----

type ResponsesResponse struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"` // "response"
	CreatedAt int64             `json:"created_at"`
	Model     string            `json:"model"`
	Status    string            `json:"status"` // completed | ...
	Output    []ResponsesOutput `json:"output"`
	Usage     *ResponsesUsage   `json:"usage,omitempty"`
}

// ResponsesOutput 输出项（message / function_call）。
type ResponsesOutput struct {
	Type    string                 `json:"type"` // message | function_call
	ID      string                 `json:"id,omitempty"`
	Role    string                 `json:"role,omitempty"`
	Status  string                 `json:"status,omitempty"`
	Content []ResponsesContentPart `json:"content,omitempty"`

	// function_call
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ---- 流式事件 ----

// ResponsesStreamEvent SSE 事件（event: response.* + data: <json>）。
type ResponsesStreamEvent struct {
	Type         string             `json:"type"`
	Delta        string             `json:"delta,omitempty"`
	Response     *ResponsesResponse `json:"response,omitempty"`
	OutputIndex  int                `json:"output_index,omitempty"`
	ContentIndex int                `json:"content_index,omitempty"`
	Item         *ResponsesOutput   `json:"item,omitempty"`
	ItemID       string             `json:"item_id,omitempty"`
}

// ResponsesErrorResponse 错误体。
type ResponsesErrorResponse struct {
	Error ResponsesError `json:"error"`
}

type ResponsesError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
