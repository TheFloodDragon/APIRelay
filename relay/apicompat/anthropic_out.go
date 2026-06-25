package apicompat

import (
	"encoding/json"
	"fmt"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// IR -> Anthropic Messages（出站序列化，中转站对外是 Anthropic 协议时使用）
// ============================================================================

// IRToAnthropicResponse 将统一响应序列化为 Anthropic 非流式响应。
func IRToAnthropicResponse(r *dto.UnifiedResponse, model string) *dto.AnthropicResponse {
	id := r.ID
	if id == "" {
		id = "msg_relay"
	}
	resp := &dto.AnthropicResponse{
		ID:         id,
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		StopReason: reverseMapStopReason(r.FinishReason),
		Usage: dto.AnthropicUsage{
			InputTokens:  r.Usage.PromptTokens,
			OutputTokens: r.Usage.CompletionTokens,
		},
	}
	if r.Content != "" {
		resp.Content = append(resp.Content, dto.AnthropicContentBlock{Type: "text", Text: r.Content})
	}
	for _, tc := range r.ToolCalls {
		input := json.RawMessage(tc.Arguments)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		resp.Content = append(resp.Content, dto.AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: input,
		})
	}
	return resp
}

func reverseMapStopReason(r string) string {
	switch r {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "":
		return "end_turn"
	default:
		return r
	}
}

// AnthropicSSEEvent 是一条待写出的 Anthropic SSE 事件。
type AnthropicSSEEvent struct {
	Event string
	Data  []byte
}

// AnthropicStreamState 维护 IR -> Anthropic 流式输出状态机。
//
// Anthropic 流式协议事件序列：
//
//	message_start -> content_block_start -> content_block_delta* ->
//	content_block_stop -> message_delta -> message_stop
type AnthropicStreamState struct {
	id           string
	model        string
	started      bool
	textOpened   bool
	finishReason string
	usage        dto.Usage
}

// NewAnthropicStreamState 创建状态机。
func NewAnthropicStreamState(id, model string) *AnthropicStreamState {
	if id == "" {
		id = "msg_relay_stream"
	}
	return &AnthropicStreamState{id: id, model: model}
}

// Begin 返回流开始时应发送的事件（message_start）。
func (s *AnthropicStreamState) Begin() []AnthropicSSEEvent {
	if s.started {
		return nil
	}
	s.started = true
	msg := dto.AnthropicResponse{
		ID:    s.id,
		Type:  "message",
		Role:  "assistant",
		Model: s.model,
		Usage: dto.AnthropicUsage{InputTokens: 0, OutputTokens: 0},
	}
	return []AnthropicSSEEvent{
		marshalEvent("message_start", dto.AnthropicStreamEvent{Type: "message_start", Message: &msg}),
		marshalEvent("ping", map[string]string{"type": "ping"}),
	}
}

// Delta 处理一个统一增量，返回应发送的 Anthropic 事件序列。
func (s *AnthropicStreamState) Delta(c *dto.UnifiedStreamChunk) []AnthropicSSEEvent {
	var events []AnthropicSSEEvent
	if !s.started {
		events = append(events, s.Begin()...)
	}

	if c.DeltaText != "" {
		if !s.textOpened {
			s.textOpened = true
			events = append(events, marshalEvent("content_block_start", dto.AnthropicStreamEvent{
				Type:         "content_block_start",
				Index:        0,
				ContentBlock: &dto.AnthropicContentBlock{Type: "text", Text: ""},
			}))
		}
		events = append(events, marshalEvent("content_block_delta", dto.AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &dto.AnthropicStreamDelta{Type: "text_delta", Text: c.DeltaText},
		}))
	}

	if c.FinishReason != "" {
		s.finishReason = c.FinishReason
	}
	if c.Usage != nil {
		s.usage = *c.Usage
	}
	return events
}

// End 返回流结束时应发送的事件（关闭文本块 + message_delta + message_stop）。
func (s *AnthropicStreamState) End() []AnthropicSSEEvent {
	var events []AnthropicSSEEvent
	if s.textOpened {
		events = append(events, marshalEvent("content_block_stop", dto.AnthropicStreamEvent{
			Type:  "content_block_stop",
			Index: 0,
		}))
	}
	stop := reverseMapStopReason(s.finishReason)
	events = append(events, marshalEvent("message_delta", dto.AnthropicStreamEvent{
		Type:  "message_delta",
		Delta: &dto.AnthropicStreamDelta{StopReason: stop},
		Usage: &dto.AnthropicUsage{
			InputTokens:  s.usage.PromptTokens,
			OutputTokens: s.usage.CompletionTokens,
		},
	}))
	events = append(events, marshalEvent("message_stop", dto.AnthropicStreamEvent{Type: "message_stop"}))
	return events
}

func marshalEvent(name string, payload any) AnthropicSSEEvent {
	b, err := json.Marshal(payload)
	if err != nil {
		b = []byte(fmt.Sprintf(`{"type":%q}`, name))
	}
	return AnthropicSSEEvent{Event: name, Data: b}
}
