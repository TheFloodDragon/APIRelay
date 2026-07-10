package apicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/dto"
)

func intPtr(i int) *int { return &i }

// TestA2_OpenAIOut_MultiToolCallIndexing 验证 OpenAI 出站对多个并行工具调用：
// index 各异、id/name 仅在各自首片出现、后续片只带 arguments。
func TestA2_OpenAIOut_MultiToolCall(t *testing.T) {
	st := NewOpenAIStreamState("id", "gpt-4o")

	// 工具0 首片（含 id/name），上游 index=0
	b0 := st.Chunk(&dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{
		{ID: "call_a", Name: "get_weather", Arguments: "", Index: intPtr(0)},
	}})
	// 工具1 首片，上游 index=1
	b1 := st.Chunk(&dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{
		{ID: "call_b", Name: "get_time", Arguments: "", Index: intPtr(1)},
	}})
	// 工具0 参数片（不应重复 id/name）
	b0d := st.Chunk(&dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{
		{Arguments: `{"city":`, Index: intPtr(0)},
	}})
	// 工具1 参数片
	b1d := st.Chunk(&dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{
		{Arguments: `{"tz":`, Index: intPtr(1)},
	}})

	tc0 := firstToolCall(t, b0)
	if tc0.Index == nil || *tc0.Index != 0 || tc0.ID != "call_a" || tc0.Function.Name != "get_weather" {
		t.Fatalf("tool0 first frame wrong: %+v", tc0)
	}
	tc1 := firstToolCall(t, b1)
	if tc1.Index == nil || *tc1.Index != 1 || tc1.ID != "call_b" || tc1.Function.Name != "get_time" {
		t.Fatalf("tool1 first frame wrong: %+v", tc1)
	}
	// 参数片：index 正确且不重复 id/name
	tc0d := firstToolCall(t, b0d)
	if tc0d.Index == nil || *tc0d.Index != 0 || tc0d.ID != "" || tc0d.Function.Name != "" {
		t.Fatalf("tool0 arg frame should omit id/name: %+v", tc0d)
	}
	if tc0d.Function.Arguments != `{"city":` {
		t.Errorf("tool0 args = %q", tc0d.Function.Arguments)
	}
	tc1d := firstToolCall(t, b1d)
	if tc1d.Index == nil || *tc1d.Index != 1 || tc1d.ID != "" {
		t.Fatalf("tool1 arg frame wrong: %+v", tc1d)
	}
}

func firstToolCall(t *testing.T, data []byte) dto.OpenAIToolCall {
	t.Helper()
	var resp dto.OpenAIChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal chunk: %v (%s)", err, data)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Delta == nil || len(resp.Choices[0].Delta.ToolCalls) == 0 {
		t.Fatalf("no tool call in chunk: %s", data)
	}
	return resp.Choices[0].Delta.ToolCalls[0]
}

// TestA2_AnthropicToOpenAI_StreamToolCall 端到端：上游 Anthropic 流式工具调用 ->
// IR -> OpenAI 出站，断言 id/name/arguments 完整重组。
func TestA2_AnthropicToOpenAI_StreamToolCall(t *testing.T) {
	p := NewAnthropicStreamParser()
	st := NewOpenAIStreamState("id", "gpt-4o")

	// 上游事件序列：两个 tool_use 块（index 1 与 2，模拟文本块占用 0 的情形）
	upstream := []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"search"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"cats\"}"}}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_2","name":"lookup"}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
	}

	tools := map[int]*collectedTool{}
	order := []int{}
	for _, line := range upstream {
		chunk, err := p.Parse([]byte(line))
		if err != nil || chunk == nil {
			continue
		}
		data := st.Chunk(chunk)
		if data == nil {
			continue
		}
		var resp dto.OpenAIChatResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Choices) == 0 || resp.Choices[0].Delta == nil {
			continue
		}
		for _, tc := range resp.Choices[0].Delta.ToolCalls {
			if tc.Index == nil {
				t.Fatalf("tool call missing index: %+v", tc)
			}
			idx := *tc.Index
			ct, ok := tools[idx]
			if !ok {
				ct = &collectedTool{}
				tools[idx] = ct
				order = append(order, idx)
			}
			if tc.ID != "" {
				ct.id = tc.ID
			}
			if tc.Function.Name != "" {
				ct.name = tc.Function.Name
			}
			ct.args += tc.Function.Arguments
		}
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tool calls, got %d (%v)", len(tools), order)
	}
	// index 0 与 1（归一后）
	t0, t1 := tools[0], tools[1]
	if t0 == nil || t1 == nil {
		t.Fatalf("normalized indices missing: %v", order)
	}
	if t0.id != "toolu_1" || t0.name != "search" || t0.args != `{"q":"cats"}` {
		t.Errorf("tool0 = %+v", t0)
	}
	if t1.id != "toolu_2" || t1.name != "lookup" || t1.args != `{}` {
		t.Errorf("tool1 = %+v", t1)
	}
}

type collectedTool struct {
	id, name, args string
}

// TestA2_OpenAIToAnthropic_StreamToolCall 端到端：上游 OpenAI 流式工具调用 ->
// IR -> Anthropic 出站，断言产出 tool_use content block 与 input_json_delta。
func TestA2_OpenAIToAnthropic_StreamToolCall(t *testing.T) {
	st := NewAnthropicStreamState("msg", "claude")

	// 上游 OpenAI 分片：一个工具调用（index 0），先 id/name 后 arguments 分两片
	upstream := []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","type":"function","function":{"name":"do_it","arguments":""}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"a\":1"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]}}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	}

	var allEvents []AnthropicSSEEvent
	for _, line := range upstream {
		chunk, err := ParseOpenAIStreamChunk([]byte(line))
		if err != nil {
			t.Fatalf("parse openai chunk: %v", err)
		}
		allEvents = append(allEvents, st.Delta(chunk)...)
	}
	allEvents = append(allEvents, st.End()...)

	var types []string
	var startBlock dto.AnthropicContentBlock
	var argsAccum strings.Builder
	for _, ev := range allEvents {
		types = append(types, ev.Event)
		switch ev.Event {
		case "content_block_start":
			var e dto.AnthropicStreamEvent
			_ = json.Unmarshal(ev.Data, &e)
			if e.ContentBlock != nil && e.ContentBlock.Type == "tool_use" {
				startBlock = *e.ContentBlock
			}
		case "content_block_delta":
			var e dto.AnthropicStreamEvent
			_ = json.Unmarshal(ev.Data, &e)
			if e.Delta != nil && e.Delta.Type == "input_json_delta" {
				argsAccum.WriteString(e.Delta.PartialJSON)
			}
		}
	}

	if startBlock.ID != "call_x" || startBlock.Name != "do_it" {
		t.Errorf("tool_use block start = %+v", startBlock)
	}
	if argsAccum.String() != `{"a":1}` {
		t.Errorf("accumulated args = %q", argsAccum.String())
	}
	if !contains(types, "content_block_start") || !contains(types, "content_block_delta") ||
		!contains(types, "content_block_stop") || !contains(types, "message_stop") {
		t.Fatalf("event types incomplete: %v", types)
	}
}
