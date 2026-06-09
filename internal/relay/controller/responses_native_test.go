package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestResponsesSSEToJSONUsesCompletedResponse(t *testing.T) {
	body := []byte(strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"ignored"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_test","object":"response","status":"completed","model":"gpt-test","output_text":"final"}}`,
		"",
	}, "\n"))

	got, err := responsesSSEToJSON(body, "fallback")
	if err != nil {
		t.Fatalf("responsesSSEToJSON returned error: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(got, &response); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if response["id"] != "resp_test" {
		t.Fatalf("id = %v, want resp_test", response["id"])
	}
	if response["output_text"] != "final" {
		t.Fatalf("output_text = %v, want final", response["output_text"])
	}
}

func TestResponsesSSEToJSONAggregatesDeltas(t *testing.T) {
	body := []byte(strings.Join([]string{
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"hello "}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"world"}`,
		"",
	}, "\n"))

	got, err := responsesSSEToJSON(body, "fallback-model")
	if err != nil {
		t.Fatalf("responsesSSEToJSON returned error: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(got, &response); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if response["model"] != "fallback-model" {
		t.Fatalf("model = %v, want fallback-model", response["model"])
	}
	if response["output_text"] != "hello world" {
		t.Fatalf("output_text = %v, want hello world", response["output_text"])
	}
}

func TestResponsesInputFunctionCallOutputToToolMessage(t *testing.T) {
	messages := responsesInputToMessages([]interface{}{
		map[string]interface{}{"type": "function_call_output", "call_id": "call_1", "output": map[string]interface{}{"result": "ok"}},
	})
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if messages[0]["role"] != "tool" || messages[0]["tool_call_id"] != "call_1" || !strings.Contains(messages[0]["content"].(string), "ok") {
		t.Fatalf("message = %+v, want tool call_1 content containing ok", messages[0])
	}
}

func TestResponsesSSEToJSONAggregatesFunctionCall(t *testing.T) {
	body := []byte(strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"in_progress","call_id":"call_1","name":"lookup","arguments":""}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"q\":"}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"\"abc\"}"}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"arguments":"{\"q\":\"abc\"}"}`,
		"",
	}, "\n"))

	got, err := responsesSSEToJSON(body, "fallback-model")
	if err != nil {
		t.Fatalf("responsesSSEToJSON returned error: %v", err)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(got, &response); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	output := response["output"].([]interface{})
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	item := output[0].(map[string]interface{})
	if item["type"] != "function_call" || item["name"] != "lookup" || !strings.Contains(item["arguments"].(string), "abc") {
		t.Fatalf("function call item = %+v, want lookup args containing abc", item)
	}
}

func TestResponsesUpstreamErrorMessage413(t *testing.T) {
	got := responsesUpstreamErrorMessage(nil, []byte("too large"), http.StatusRequestEntityTooLarge)
	if !strings.Contains(got, "请求体过大") {
		t.Fatalf("message = %q, want request-too-large hint", got)
	}
}
