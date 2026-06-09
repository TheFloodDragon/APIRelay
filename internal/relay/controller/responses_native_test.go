package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestResponsesRequestToChatCompletionsUsesCompletionTokensForGPT5(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":"hello","max_output_tokens":123,"parallel_tool_calls":true,"tool_choice":"auto"}`)
	got, modelName, stream, err := responsesRequestToChatCompletions(body, false)
	if err != nil {
		t.Fatalf("responsesRequestToChatCompletions returned error: %v", err)
	}
	if modelName != "gpt-5.5" {
		t.Fatalf("model = %q, want gpt-5.5", modelName)
	}
	if stream {
		t.Fatalf("stream = true, want false")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["max_completion_tokens"] != float64(123) {
		t.Fatalf("max_completion_tokens = %v, want 123; body = %s", payload["max_completion_tokens"], string(got))
	}
	if _, exists := payload["max_tokens"]; exists {
		t.Fatalf("max_tokens should be absent for gpt-5 series; body = %s", string(got))
	}
	if _, exists := payload["parallel_tool_calls"]; exists {
		t.Fatalf("parallel_tool_calls should be absent without tools; body = %s", string(got))
	}
	if _, exists := payload["tool_choice"]; exists {
		t.Fatalf("tool_choice should be absent without tools; body = %s", string(got))
	}
}

func TestResponsesRequestToChatCompletionsAddsStreamUsage(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","input":"hello","stream":true,"stream_options":{"foo":"bar"}}`)
	got, _, stream, err := responsesRequestToChatCompletions(body, false)
	if err != nil {
		t.Fatalf("responsesRequestToChatCompletions returned error: %v", err)
	}
	if !stream {
		t.Fatalf("stream = false, want true")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	streamOptions, ok := payload["stream_options"].(map[string]interface{})
	if !ok {
		t.Fatalf("stream_options missing or invalid: %s", string(got))
	}
	if streamOptions["include_usage"] != true || streamOptions["foo"] != "bar" {
		t.Fatalf("stream_options = %+v, want include_usage=true and foo preserved", streamOptions)
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

func TestWriteFinalResponsesErrorPreservesUpstream4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)

	writeFinalResponsesError(ctx, nil, "model not found", true, http.StatusNotFound)

	if writer.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", writer.Code, http.StatusNotFound, writer.Body.String())
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(writer.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["error"]["type"] != "upstream_error" {
		t.Fatalf("error type = %v, want upstream_error", payload["error"]["type"])
	}
	if payload["error"]["details"] != "model not found" {
		t.Fatalf("details = %v, want model not found", payload["error"]["details"])
	}
}

func TestWriteFinalResponsesErrorKeeps413SpecificType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)

	writeFinalResponsesError(ctx, nil, "too large", true, http.StatusRequestEntityTooLarge)

	if writer.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body = %s", writer.Code, http.StatusRequestEntityTooLarge, writer.Body.String())
	}
	var payload map[string]map[string]interface{}
	if err := json.Unmarshal(writer.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["error"]["type"] != "request_too_large" {
		t.Fatalf("error type = %v, want request_too_large", payload["error"]["type"])
	}
}
