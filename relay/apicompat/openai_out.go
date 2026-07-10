package apicompat

import (
	"encoding/json"
	"time"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// IR -> OpenAI 出站序列化（中转站对外是 OpenAI 协议时使用）
// ============================================================================

// IRToOpenAIResponse 将统一响应序列化为 OpenAI 非流式响应。
func IRToOpenAIResponse(r *dto.UnifiedResponse, model string) *dto.OpenAIChatResponse {
	id := r.ID
	if id == "" {
		id = "chatcmpl-" + r.ID
	}
	content, _ := json.Marshal(r.Content)
	msg := &dto.OpenAIMessage{Role: "assistant", Content: content}
	for _, tc := range r.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, dto.OpenAIToolCall{
			ID:       tc.ID,
			Type:     "function",
			Function: dto.OpenAIToolCallFunc{Name: tc.Name, Arguments: tc.Arguments},
		})
	}
	finish := r.FinishReason
	if finish == "" {
		finish = "stop"
	}
	return &dto.OpenAIChatResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []dto.OpenAIChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: &finish,
		}},
		Usage: &dto.OpenAIUsage{
			PromptTokens:     r.Usage.PromptTokens,
			CompletionTokens: r.Usage.CompletionTokens,
			TotalTokens:      r.Usage.TotalTokens,
		},
	}
}

// OpenAIStreamState 维护 IR->OpenAI 流式输出的状态。
type OpenAIStreamState struct {
	id      string
	model   string
	created int64
	started bool
	// toolIndex 将上游工具调用的稳定 index 归一为 0 基连续序号（OpenAI 客户端惯例）。
	toolIndex map[int]int
	// toolStarted 记录某归一 index 是否已发过 id/name（后续分片只发 arguments）。
	toolStarted map[int]bool
	nextToolIdx int
}

// NewOpenAIStreamState 创建流式状态。
func NewOpenAIStreamState(id, model string) *OpenAIStreamState {
	if id == "" {
		id = "chatcmpl-stream"
	}
	return &OpenAIStreamState{
		id:          id,
		model:       model,
		created:     time.Now().Unix(),
		toolIndex:   make(map[int]int),
		toolStarted: make(map[int]bool),
	}
}

// normalizeToolIdx 将上游稳定 index 映射为 0 基连续序号。无 index 时按出现顺序分配。
func (s *OpenAIStreamState) normalizeToolIdx(upstream *int) int {
	key := -1
	if upstream != nil {
		key = *upstream
	}
	if idx, ok := s.toolIndex[key]; ok {
		return idx
	}
	idx := s.nextToolIdx
	s.nextToolIdx++
	s.toolIndex[key] = idx
	return idx
}

// Chunk 将统一增量序列化为一条 OpenAI SSE chunk 的 JSON（不含 "data: "）。
// 返回 nil 表示该增量无需输出。
func (s *OpenAIStreamState) Chunk(c *dto.UnifiedStreamChunk) []byte {
	delta := &dto.OpenAIDelta{}
	if !s.started {
		delta.Role = "assistant"
		s.started = true
	}
	delta.Content = c.DeltaText
	for _, tc := range c.ToolCalls {
		idx := s.normalizeToolIdx(tc.Index)
		outTC := dto.OpenAIToolCall{
			Type:     "function",
			Index:    &idx,
			Function: dto.OpenAIToolCallFunc{Arguments: tc.Arguments},
		}
		// 仅在该工具调用首个分片发送 id/name，避免逐片重复。
		if !s.toolStarted[idx] {
			s.toolStarted[idx] = true
			outTC.ID = tc.ID
			outTC.Function.Name = tc.Name
		}
		delta.ToolCalls = append(delta.ToolCalls, outTC)
	}

	choice := dto.OpenAIChoice{Index: 0, Delta: delta}
	if c.FinishReason != "" {
		fr := c.FinishReason
		choice.FinishReason = &fr
	}
	resp := dto.OpenAIChatResponse{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: s.created,
		Model:   s.model,
		Choices: []dto.OpenAIChoice{choice},
	}
	if c.Usage != nil {
		resp.Usage = &dto.OpenAIUsage{
			PromptTokens:     c.Usage.PromptTokens,
			CompletionTokens: c.Usage.CompletionTokens,
			TotalTokens:      c.Usage.TotalTokens,
		}
	}
	b, _ := json.Marshal(resp)
	return b
}
