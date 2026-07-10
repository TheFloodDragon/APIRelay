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
//	message_start -> (content_block_start -> content_block_delta* ->
//	content_block_stop)* -> message_delta -> message_stop
//
// 支持文本块与 tool_use 块。为满足 Anthropic「同一时刻至多一个块打开」的约束，
// 切换到新块前会关闭当前块（跨协议重组为 best-effort，覆盖 OpenAI 常见的
// 先文本后工具、或纯工具的分片顺序）。
type AnthropicStreamState struct {
	id           string
	model        string
	started      bool
	finishReason string
	usage        dto.Usage

	openIndex int         // 当前打开的块索引，-1 表示无
	nextBlock int         // 下一个可分配的块索引
	textBlock int         // 文本块索引，-1 表示尚未创建
	toolBlock map[int]int // 上游工具 index key -> anthropic 块索引
}

// NewAnthropicStreamState 创建状态机。
func NewAnthropicStreamState(id, model string) *AnthropicStreamState {
	if id == "" {
		id = "msg_relay_stream"
	}
	return &AnthropicStreamState{
		id:        id,
		model:     model,
		openIndex: -1,
		textBlock: -1,
		toolBlock: make(map[int]int),
	}
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

// closeCurrent 关闭当前打开的块（若有），返回 content_block_stop 事件。
func (s *AnthropicStreamState) closeCurrent() []AnthropicSSEEvent {
	if s.openIndex < 0 {
		return nil
	}
	ev := marshalEvent("content_block_stop", dto.AnthropicStreamEvent{
		Type:  "content_block_stop",
		Index: s.openIndex,
	})
	s.openIndex = -1
	return []AnthropicSSEEvent{ev}
}

func toolIndexKey(idx *int) int {
	if idx == nil {
		return -1
	}
	return *idx
}

// Delta 处理一个统一增量，返回应发送的 Anthropic 事件序列。
func (s *AnthropicStreamState) Delta(c *dto.UnifiedStreamChunk) []AnthropicSSEEvent {
	var events []AnthropicSSEEvent
	if !s.started {
		events = append(events, s.Begin()...)
	}

	if c.DeltaText != "" {
		// 确保文本块打开（必要时先关闭其它块）。
		if s.textBlock < 0 {
			events = append(events, s.closeCurrent()...)
			s.textBlock = s.nextBlock
			s.nextBlock++
			s.openIndex = s.textBlock
			events = append(events, marshalEvent("content_block_start", dto.AnthropicStreamEvent{
				Type:         "content_block_start",
				Index:        s.textBlock,
				ContentBlock: &dto.AnthropicContentBlock{Type: "text", Text: ""},
			}))
		} else if s.openIndex != s.textBlock {
			events = append(events, s.closeCurrent()...)
			s.openIndex = s.textBlock
		}
		events = append(events, marshalEvent("content_block_delta", dto.AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: s.textBlock,
			Delta: &dto.AnthropicStreamDelta{Type: "text_delta", Text: c.DeltaText},
		}))
	}

	for _, tc := range c.ToolCalls {
		key := toolIndexKey(tc.Index)
		blk, ok := s.toolBlock[key]
		if !ok {
			// 新工具块：关闭当前块并开启 tool_use 块（携带 id/name）。
			events = append(events, s.closeCurrent()...)
			blk = s.nextBlock
			s.nextBlock++
			s.toolBlock[key] = blk
			s.openIndex = blk
			input := json.RawMessage("{}")
			events = append(events, marshalEvent("content_block_start", dto.AnthropicStreamEvent{
				Type:  "content_block_start",
				Index: blk,
				ContentBlock: &dto.AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				},
			}))
		} else if s.openIndex != blk {
			events = append(events, s.closeCurrent()...)
			s.openIndex = blk
		}
		if tc.Arguments != "" {
			events = append(events, marshalEvent("content_block_delta", dto.AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: blk,
				Delta: &dto.AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: tc.Arguments},
			}))
		}
	}

	if c.FinishReason != "" {
		s.finishReason = c.FinishReason
	}
	if c.Usage != nil {
		s.usage = *c.Usage
	}
	return events
}

// End 返回流结束时应发送的事件（关闭当前块 + message_delta + message_stop）。
func (s *AnthropicStreamState) End() []AnthropicSSEEvent {
	var events []AnthropicSSEEvent
	events = append(events, s.closeCurrent()...)
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
