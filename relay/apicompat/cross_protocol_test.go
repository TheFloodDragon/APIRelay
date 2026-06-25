package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/apirelay/apirelay/dto"
)

// 测试 Anthropic 请求 -> IR -> OpenAI 上游请求（跨协议：对外 Anthropic，上游 OpenAI）。
func TestAnthropicToIRToOpenAI(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"max_tokens": 100,
		"system": "be brief",
		"messages": [
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": [{"type":"text","text":"hi there"}]}
		]
	}`)
	ir, err := ParseAnthropicRequest(body)
	if err != nil {
		t.Fatalf("parse anthropic: %v", err)
	}
	if ir.System != "be brief" {
		t.Errorf("system = %q", ir.System)
	}
	if ir.MaxTokens == nil || *ir.MaxTokens != 100 {
		t.Errorf("max_tokens = %v", ir.MaxTokens)
	}
	if len(ir.Messages) != 2 {
		t.Fatalf("messages = %d", len(ir.Messages))
	}

	// 转为 OpenAI 上游请求
	oa := BuildOpenAIRequest(ir, "gpt-4o")
	if oa.Model != "gpt-4o" {
		t.Errorf("upstream model = %q", oa.Model)
	}
	if oa.Messages[0].Role != "system" {
		t.Errorf("first openai msg should be system, got %q", oa.Messages[0].Role)
	}
}

// 测试 OpenAI 请求 -> IR -> Anthropic 上游请求（跨协议：对外 OpenAI，上游 Anthropic）。
func TestOpenAIToIRToAnthropic(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "sys"},
			{"role": "user", "content": "hi"}
		]
	}`)
	ir, err := ParseOpenAIRequest(body)
	if err != nil {
		t.Fatalf("parse openai: %v", err)
	}
	an := BuildAnthropicRequest(ir, "claude-3-5-haiku")
	if an.Model != "claude-3-5-haiku" {
		t.Errorf("upstream model = %q", an.Model)
	}
	if an.MaxTokens <= 0 {
		t.Error("anthropic max_tokens must be > 0 (required field)")
	}
	var sys string
	_ = json.Unmarshal(an.System, &sys)
	if sys != "sys" {
		t.Errorf("system = %q", sys)
	}
	if len(an.Messages) != 1 || an.Messages[0].Role != "user" {
		t.Fatalf("messages = %+v", an.Messages)
	}
}

// 测试 Anthropic 响应 -> IR -> OpenAI 出站响应。
func TestAnthropicResponseToOpenAIOut(t *testing.T) {
	resp := &dto.AnthropicResponse{
		ID:         "msg_1",
		Model:      "claude",
		StopReason: "end_turn",
		Content: []dto.AnthropicContentBlock{
			{Type: "text", Text: "Hello world"},
		},
		Usage: dto.AnthropicUsage{InputTokens: 10, OutputTokens: 5},
	}
	ir := AnthropicResponseToIR(resp)
	if ir.Content != "Hello world" {
		t.Errorf("content = %q", ir.Content)
	}
	if ir.FinishReason != "stop" {
		t.Errorf("finish = %q (end_turn should map to stop)", ir.FinishReason)
	}
	if ir.Usage.TotalTokens != 15 {
		t.Errorf("total tokens = %d", ir.Usage.TotalTokens)
	}

	oa := IRToOpenAIResponse(ir, "gpt-4o")
	if oa.Choices[0].Message == nil {
		t.Fatal("missing message")
	}
	var content string
	_ = json.Unmarshal(oa.Choices[0].Message.Content, &content)
	if content != "Hello world" {
		t.Errorf("openai content = %q", content)
	}
}

// 测试 Anthropic 流式事件 -> IR 增量。
func TestAnthropicStreamParser(t *testing.T) {
	p := NewAnthropicStreamParser()

	// message_start 带 input tokens
	_, _ = p.Parse([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":7,"output_tokens":0}}}`))

	// 文本增量
	chunk, _ := p.Parse([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`))
	if chunk == nil || chunk.DeltaText != "Hi" {
		t.Fatalf("text delta = %+v", chunk)
	}

	// message_delta 带 stop_reason 与 usage
	chunk, _ = p.Parse([]byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`))
	if chunk == nil || chunk.FinishReason != "stop" {
		t.Fatalf("finish = %+v", chunk)
	}
	if chunk.Usage == nil || chunk.Usage.PromptTokens != 7 || chunk.Usage.CompletionTokens != 4 {
		t.Errorf("usage = %+v", chunk.Usage)
	}
}

// 测试 IR -> Anthropic 出站流式事件序列。
func TestAnthropicStreamStateLifecycle(t *testing.T) {
	st := NewAnthropicStreamState("msg_x", "claude")
	// 第一个文本增量应包含 message_start + content_block_start + delta
	events := st.Delta(&dto.UnifiedStreamChunk{DeltaText: "Hi"})
	var types []string
	for _, e := range events {
		types = append(types, e.Event)
	}
	if !contains(types, "message_start") || !contains(types, "content_block_start") || !contains(types, "content_block_delta") {
		t.Fatalf("first delta events = %v", types)
	}

	// 结束应包含 content_block_stop + message_delta + message_stop
	st.Delta(&dto.UnifiedStreamChunk{FinishReason: "stop", Usage: &dto.Usage{PromptTokens: 3, CompletionTokens: 2}})
	endEvents := st.End()
	var endTypes []string
	for _, e := range endEvents {
		endTypes = append(endTypes, e.Event)
	}
	if !contains(endTypes, "content_block_stop") || !contains(endTypes, "message_delta") || !contains(endTypes, "message_stop") {
		t.Fatalf("end events = %v", endTypes)
	}
}

// 测试 Responses 请求 -> IR -> OpenAI 上游。
func TestResponsesToIRToOpenAI(t *testing.T) {
	body := []byte(`{
		"model": "o1",
		"instructions": "reason carefully",
		"input": "what is 2+2"
	}`)
	ir, err := ParseResponsesRequest(body)
	if err != nil {
		t.Fatalf("parse responses: %v", err)
	}
	if ir.System != "reason carefully" {
		t.Errorf("system = %q", ir.System)
	}
	if len(ir.Messages) != 1 || ir.Messages[0].Content != "what is 2+2" {
		t.Fatalf("messages = %+v", ir.Messages)
	}
}

// 测试 IR -> Responses 出站响应。
func TestIRToResponsesOut(t *testing.T) {
	ir := &dto.UnifiedResponse{
		ID:      "r1",
		Content: "four",
		Usage:   dto.Usage{PromptTokens: 5, CompletionTokens: 1, TotalTokens: 6},
	}
	resp := IRToResponsesResponse(ir, "o1")
	if resp.Object != "response" || resp.Status != "completed" {
		t.Errorf("resp meta = %+v", resp)
	}
	if len(resp.Output) != 1 || resp.Output[0].Content[0].Text != "four" {
		t.Fatalf("output = %+v", resp.Output)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
