package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGeminiRequestToProtocolPreservesToolsAndFunctionParts(t *testing.T) {
	body := []byte(`{
		"tools":[{"functionDeclarations":[{"name":"lookup","description":"Lookup data","parameters":{"type":"object","properties":{"q":{"type":"string"}}}}]}],
		"toolConfig":{"functionCallingConfig":{"mode":"ANY","allowedFunctionNames":["lookup"]}},
		"contents":[
			{"role":"user","parts":[{"text":"find it"}]},
			{"role":"model","parts":[{"functionCall":{"id":"call_1","name":"lookup","args":{"q":"abc"}}}]},
			{"role":"user","parts":[{"functionResponse":{"id":"call_1","name":"lookup","response":{"result":"ok"}}}]}
		]
	}`)

	req, err := GeminiGenerateContentRequestToProtocol(body, "gemini-test", false)
	if err != nil {
		t.Fatalf("GeminiGenerateContentRequestToProtocol returned error: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(req.Tools))
	}
	if len(req.Messages) != 3 {
		t.Fatalf("messages len = %d, want 3", len(req.Messages))
	}
	if len(req.Messages[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls len = %d, want 1", len(req.Messages[1].ToolCalls))
	}
	name, arguments := toolCallNameAndArguments(req.Messages[1].ToolCalls[0])
	if name != "lookup" || !strings.Contains(arguments, "abc") {
		t.Fatalf("tool call = %q %q, want lookup args containing abc", name, arguments)
	}
	if req.Messages[2].Role != "tool" || req.Messages[2].ToolCallID != "call_1" || !strings.Contains(req.Messages[2].Content, "ok") {
		t.Fatalf("tool response message = %+v, want tool call_1 content containing ok", req.Messages[2])
	}
}

func TestProtocolToGeminiRequestPreservesToolsAndFunctionParts(t *testing.T) {
	req := &ChatRequest{
		Model:      "gemini-test",
		Tools:      []map[string]interface{}{{"type": "function", "function": map[string]interface{}{"name": "lookup", "description": "Lookup data", "parameters": map[string]interface{}{"type": "object"}}}},
		ToolChoice: map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "lookup"}},
		Messages: []ChatMessage{
			{Role: "user", Content: "find it"},
			{Role: "assistant", ToolCalls: []map[string]interface{}{{"id": "call_1", "type": "function", "function": map[string]interface{}{"name": "lookup", "arguments": `{"q":"abc"}`}}}},
			{Role: "tool", ToolCallID: "call_1", Content: `{"result":"ok"}`},
		},
	}

	body, err := ProtocolToGeminiGenerateContentRequest(req)
	if err != nil {
		t.Fatalf("ProtocolToGeminiGenerateContentRequest returned error: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := raw["tools"].([]interface{}); !ok {
		t.Fatalf("tools missing from Gemini request: %s", string(body))
	}
	encoded := string(body)
	for _, want := range []string{"functionDeclarations", "functionCall", "functionResponse", "lookup", "call_1"} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("Gemini request missing %q: %s", want, encoded)
		}
	}
}

func TestGeminiResponseToProtocolPreservesFunctionCall(t *testing.T) {
	body := []byte(`{"candidates":[{"index":0,"finishReason":"FUNCTION_CALL","content":{"role":"model","parts":[{"functionCall":{"id":"call_1","name":"lookup","args":{"q":"abc"}}}]}}]}`)
	resp, err := GeminiGenerateContentResponseToProtocol(body)
	if err != nil {
		t.Fatalf("GeminiGenerateContentResponseToProtocol returned error: %v", err)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("finish reason = %q, want tool_calls", resp.FinishReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls len = %d, want 1", len(resp.ToolCalls))
	}
}

func TestOpenAIStreamEventsPreserveToolCallDelta(t *testing.T) {
	data := `{"id":"chatcmpl_1","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]},"finish_reason":null}]}`
	events, err := OpenAIStreamEventsFromData(data)
	if err != nil {
		t.Fatalf("OpenAIStreamEventsFromData returned error: %v", err)
	}
	if len(events) != 1 || len(events[0].ToolCalls) != 1 {
		t.Fatalf("events = %+v, want one tool call delta", events)
	}
	chunk, err := ProtocolStreamEventToOpenAIData(events[0])
	if err != nil {
		t.Fatalf("ProtocolStreamEventToOpenAIData returned error: %v", err)
	}
	if !strings.Contains(string(chunk), "tool_calls") || !strings.Contains(string(chunk), "lookup") {
		t.Fatalf("encoded OpenAI chunk missing tool call: %s", string(chunk))
	}
}
