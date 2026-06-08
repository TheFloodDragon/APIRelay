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

func TestResponsesUpstreamErrorMessage413(t *testing.T) {
	got := responsesUpstreamErrorMessage(nil, []byte("too large"), http.StatusRequestEntityTooLarge)
	if !strings.Contains(got, "请求体过大") {
		t.Fatalf("message = %q, want request-too-large hint", got)
	}
}
