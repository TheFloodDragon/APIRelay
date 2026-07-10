package apicompat

import (
	"encoding/json"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// Anthropic 流式事件 -> IR 增量（上游为 Anthropic 时使用）
// ============================================================================

// AnthropicStreamParser 维护跨事件的解析状态（如累积 usage、工具调用映射）。
type AnthropicStreamParser struct {
	inputTokens  int
	outputTokens int
	// 记录每个 content block index 对应的工具调用元信息
	toolBlocks map[int]*toolBlockState
}

type toolBlockState struct {
	id   string
	name string
}

// NewAnthropicStreamParser 创建解析器。
func NewAnthropicStreamParser() *AnthropicStreamParser {
	return &AnthropicStreamParser{toolBlocks: make(map[int]*toolBlockState)}
}

// Parse 解析一条 Anthropic SSE data（JSON）为统一增量。
// 返回 nil chunk 表示该事件无需向下游输出。
func (p *AnthropicStreamParser) Parse(data []byte) (*dto.UnifiedStreamChunk, error) {
	var ev dto.AnthropicStreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, err
	}

	switch ev.Type {
	case "message_start":
		if ev.Message != nil {
			p.inputTokens = ev.Message.Usage.InputTokens
			p.outputTokens = ev.Message.Usage.OutputTokens
		}
		return nil, nil

	case "content_block_start":
		if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
			p.toolBlocks[ev.Index] = &toolBlockState{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
			idx := ev.Index
			return &dto.UnifiedStreamChunk{
				ToolCalls: []dto.UnifiedToolCall{{
					ID:        ev.ContentBlock.ID,
					Name:      ev.ContentBlock.Name,
					Arguments: "",
					Index:     &idx,
				}},
			}, nil
		}
		return nil, nil

	case "content_block_delta":
		if ev.Delta == nil {
			return nil, nil
		}
		switch ev.Delta.Type {
		case "text_delta":
			return &dto.UnifiedStreamChunk{DeltaText: ev.Delta.Text}, nil
		case "input_json_delta":
			ts := p.toolBlocks[ev.Index]
			idx := ev.Index
			tc := dto.UnifiedToolCall{Arguments: ev.Delta.PartialJSON, Index: &idx}
			if ts != nil {
				tc.ID = ts.id
				tc.Name = ts.name
			}
			return &dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{tc}}, nil
		}
		return nil, nil

	case "message_delta":
		chunk := &dto.UnifiedStreamChunk{}
		if ev.Delta != nil && ev.Delta.StopReason != "" {
			chunk.FinishReason = mapAnthropicStopReason(ev.Delta.StopReason)
		}
		if ev.Usage != nil {
			p.outputTokens = ev.Usage.OutputTokens
		}
		chunk.Usage = &dto.Usage{
			PromptTokens:     p.inputTokens,
			CompletionTokens: p.outputTokens,
			TotalTokens:      p.inputTokens + p.outputTokens,
		}
		return chunk, nil

	case "message_stop":
		return &dto.UnifiedStreamChunk{Done: true}, nil

	default: // ping / content_block_stop 等
		return nil, nil
	}
}
