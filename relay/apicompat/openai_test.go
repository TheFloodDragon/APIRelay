package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/apirelay/apirelay/dto"
)

func TestParseOpenAIRequest_BasicAndSystem(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"stream": true,
		"messages": [
			{"role": "system", "content": "you are helpful"},
			{"role": "user", "content": "hi"}
		]
	}`)
	ir, err := ParseOpenAIRequest(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if ir.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", ir.Model)
	}
	if !ir.Stream {
		t.Error("stream should be true")
	}
	if ir.System != "you are helpful" {
		t.Errorf("system = %q", ir.System)
	}
	// system 已抽出，messages 仅剩 user
	if len(ir.Messages) != 1 || ir.Messages[0].Role != dto.RoleUser {
		t.Fatalf("messages = %+v", ir.Messages)
	}
	if ir.Messages[0].Content != "hi" {
		t.Errorf("user content = %q", ir.Messages[0].Content)
	}
}

func TestParseOpenAIRequest_MultimodalContent(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": [
				{"type": "text", "text": "look"},
				{"type": "image_url", "image_url": {"url": "http://x/y.png"}}
			]}
		]
	}`)
	ir, err := ParseOpenAIRequest(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	m := ir.Messages[0]
	if m.Content != "look" {
		t.Errorf("accum text = %q", m.Content)
	}
	if len(m.Parts) != 2 || m.Parts[1].ImageURL != "http://x/y.png" {
		t.Errorf("parts = %+v", m.Parts)
	}
}

func TestBuildOpenAIRequest_RoundTrip(t *testing.T) {
	maxTokens := 100
	ir := &dto.UnifiedRequest{
		Model:     "origin-model",
		System:    "sys",
		MaxTokens: &maxTokens,
		Stream:    true,
		Messages: []dto.UnifiedMessage{
			{Role: dto.RoleUser, Content: "hello"},
		},
	}
	out := BuildOpenAIRequest(ir, "upstream-model")
	if out.Model != "upstream-model" {
		t.Errorf("model = %q, want upstream-model", out.Model)
	}
	if out.StreamOptions == nil || !out.StreamOptions.IncludeUsage {
		t.Error("stream should request usage")
	}
	// 首条应为 system
	if out.Messages[0].Role != "system" {
		t.Fatalf("first message role = %q", out.Messages[0].Role)
	}
	var sys string
	_ = json.Unmarshal(out.Messages[0].Content, &sys)
	if sys != "sys" {
		t.Errorf("system content = %q", sys)
	}
}

func TestParseOpenAIStreamChunk_Usage(t *testing.T) {
	data := []byte(`{"choices":[{"delta":{"content":"hi"},"finish_reason":null}],"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}}`)
	chunk, err := ParseOpenAIStreamChunk(data)
	if err != nil {
		t.Fatalf("parse chunk: %v", err)
	}
	if chunk.DeltaText != "hi" {
		t.Errorf("delta = %q", chunk.DeltaText)
	}
	if chunk.Usage == nil || chunk.Usage.TotalTokens != 8 {
		t.Errorf("usage = %+v", chunk.Usage)
	}
}

func TestOpenAIStreamState_Chunk(t *testing.T) {
	st := NewOpenAIStreamState("id-1", "model-x")
	// 第一次应带 role=assistant
	first := st.Chunk(&dto.UnifiedStreamChunk{DeltaText: "a"})
	var resp dto.OpenAIChatResponse
	if err := json.Unmarshal(first, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Object != "chat.completion.chunk" {
		t.Errorf("object = %q", resp.Object)
	}
	if resp.Choices[0].Delta.Role != "assistant" {
		t.Errorf("first chunk should set role assistant, got %q", resp.Choices[0].Delta.Role)
	}
	// 第二次不应再带 role
	second := st.Chunk(&dto.UnifiedStreamChunk{DeltaText: "b"})
	var resp2 dto.OpenAIChatResponse
	_ = json.Unmarshal(second, &resp2)
	if resp2.Choices[0].Delta.Role != "" {
		t.Errorf("second chunk should not set role, got %q", resp2.Choices[0].Delta.Role)
	}
}
