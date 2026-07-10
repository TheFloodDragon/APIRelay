package apicompat

import (
	"encoding/json"
	"testing"
)

// TestA6_AnthropicImageToOpenAI 跨协议：Anthropic 带图请求 -> IR -> OpenAI 上游，
// content 应为多模态数组且含 image_url，不被降级为纯文本。
func TestA6_AnthropicImageToOpenAI(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"max_tokens": 100,
		"messages": [
			{"role":"user","content":[
				{"type":"text","text":"what is this"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAA"}}
			]}
		]
	}`)
	ir, err := ParseAnthropicRequest(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	oa := BuildOpenAIRequest(ir, "gpt-4o")
	if len(oa.Messages) != 1 {
		t.Fatalf("messages = %d", len(oa.Messages))
	}
	// content 应为数组
	var parts []map[string]any
	if err := json.Unmarshal(oa.Messages[0].Content, &parts); err != nil {
		t.Fatalf("content should be multimodal array, got %s: %v", oa.Messages[0].Content, err)
	}
	var hasText, hasImage bool
	for _, p := range parts {
		switch p["type"] {
		case "text":
			hasText = true
		case "image_url":
			hasImage = true
			img, ok := p["image_url"].(map[string]any)
			if !ok || img["url"] == "" {
				t.Errorf("image_url missing url: %v", p)
			}
			if url, _ := img["url"].(string); url == "" ||
				(len(url) < 5 || url[:5] != "data:") {
				t.Errorf("expected data URL, got %v", img["url"])
			}
		}
	}
	if !hasText || !hasImage {
		t.Errorf("multimodal content incomplete: text=%v image=%v (%s)", hasText, hasImage, oa.Messages[0].Content)
	}
}

// TestA6_AnthropicImageToResponses 跨协议：Anthropic 带图 -> IR -> Responses 上游，
// input 应含 input_image。
func TestA6_AnthropicImageToResponses(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet",
		"max_tokens": 100,
		"messages": [
			{"role":"user","content":[
				{"type":"text","text":"caption"},
				{"type":"image","source":{"type":"url","url":"https://x/y.png"}}
			]}
		]
	}`)
	ir, err := ParseAnthropicRequest(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rq := BuildResponsesRequest(ir, "o1")
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(rq.Input, &items); err != nil {
		t.Fatalf("input not array: %v", err)
	}
	// 找到 user message 的 content 数组，断言含 input_image
	var found bool
	for _, it := range items {
		var content []map[string]any
		if err := json.Unmarshal(it["content"], &content); err != nil {
			continue
		}
		for _, p := range content {
			if p["type"] == "input_image" {
				found = true
				if p["image_url"] != "https://x/y.png" {
					t.Errorf("input_image url = %v", p["image_url"])
				}
			}
		}
	}
	if !found {
		t.Errorf("responses input should contain input_image, got %s", rq.Input)
	}
}

// TestA6_PlainTextUnchanged 无 parts 的纯文本消息仍序列化为字符串 content。
func TestA6_PlainTextUnchanged(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
	ir, err := ParseOpenAIRequest(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	oa := BuildOpenAIRequest(ir, "gpt-4o")
	var s string
	if err := json.Unmarshal(oa.Messages[0].Content, &s); err != nil {
		t.Fatalf("plain content should be string, got %s: %v", oa.Messages[0].Content, err)
	}
	if s != "hi" {
		t.Errorf("content = %q", s)
	}
}
