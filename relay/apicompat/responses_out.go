package apicompat

import (
	"encoding/json"
	"time"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// IR -> OpenAI Responses（出站序列化，中转站对外是 Responses 协议时使用）
// ============================================================================

// IRToResponsesResponse 将统一响应序列化为 Responses 非流式响应。
func IRToResponsesResponse(r *dto.UnifiedResponse, model string) *dto.ResponsesResponse {
	id := r.ID
	if id == "" {
		id = "resp_relay"
	}
	resp := &dto.ResponsesResponse{
		ID:        id,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     model,
		Status:    "completed",
		Usage: &dto.ResponsesUsage{
			InputTokens:  r.Usage.PromptTokens,
			OutputTokens: r.Usage.CompletionTokens,
			TotalTokens:  r.Usage.TotalTokens,
		},
	}
	if r.Content != "" {
		resp.Output = append(resp.Output, dto.ResponsesOutput{
			Type:   "message",
			ID:     "msg_0",
			Role:   "assistant",
			Status: "completed",
			Content: []dto.ResponsesContentPart{
				{Type: "output_text", Text: r.Content},
			},
		})
	}
	for i, tc := range r.ToolCalls {
		resp.Output = append(resp.Output, dto.ResponsesOutput{
			Type:      "function_call",
			ID:        funcCallID(i),
			CallID:    tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
			Status:    "completed",
		})
	}
	return resp
}

func funcCallID(i int) string {
	return "fc_" + string(rune('0'+i))
}

// ResponsesSSEEvent 一条待写出的 Responses SSE 事件。
type ResponsesSSEEvent struct {
	Event string
	Data  []byte
}

// ResponsesStreamState 维护 IR -> Responses 流式输出状态机。
type ResponsesStreamState struct {
	id      string
	model   string
	started bool
	usage   dto.Usage
	content string
}

// NewResponsesStreamState 创建状态机。
func NewResponsesStreamState(id, model string) *ResponsesStreamState {
	if id == "" {
		id = "resp_relay_stream"
	}
	return &ResponsesStreamState{id: id, model: model}
}

// Begin 返回 response.created / response.in_progress 事件。
func (s *ResponsesStreamState) Begin() []ResponsesSSEEvent {
	if s.started {
		return nil
	}
	s.started = true
	base := &dto.ResponsesResponse{
		ID:        s.id,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     s.model,
		Status:    "in_progress",
	}
	return []ResponsesSSEEvent{
		marshalResponsesEvent("response.created", dto.ResponsesStreamEvent{Type: "response.created", Response: base}),
	}
}

// Delta 处理统一增量，返回 Responses 事件序列。
func (s *ResponsesStreamState) Delta(c *dto.UnifiedStreamChunk) []ResponsesSSEEvent {
	var events []ResponsesSSEEvent
	if !s.started {
		events = append(events, s.Begin()...)
	}
	if c.DeltaText != "" {
		s.content += c.DeltaText
		events = append(events, marshalResponsesEvent("response.output_text.delta", dto.ResponsesStreamEvent{
			Type:        "response.output_text.delta",
			OutputIndex: 0,
			Delta:       c.DeltaText,
		}))
	}
	if c.Usage != nil {
		s.usage = *c.Usage
	}
	return events
}

// End 返回 response.completed 事件。
func (s *ResponsesStreamState) End() []ResponsesSSEEvent {
	final := &dto.ResponsesResponse{
		ID:        s.id,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     s.model,
		Status:    "completed",
		Usage: &dto.ResponsesUsage{
			InputTokens:  s.usage.PromptTokens,
			OutputTokens: s.usage.CompletionTokens,
			TotalTokens:  s.usage.TotalTokens,
		},
	}
	if s.content != "" {
		final.Output = append(final.Output, dto.ResponsesOutput{
			Type:    "message",
			ID:      "msg_0",
			Role:    "assistant",
			Status:  "completed",
			Content: []dto.ResponsesContentPart{{Type: "output_text", Text: s.content}},
		})
	}
	return []ResponsesSSEEvent{
		marshalResponsesEvent("response.completed", dto.ResponsesStreamEvent{Type: "response.completed", Response: final}),
	}
}

func marshalResponsesEvent(name string, payload any) ResponsesSSEEvent {
	b, _ := json.Marshal(payload)
	return ResponsesSSEEvent{Event: name, Data: b}
}
