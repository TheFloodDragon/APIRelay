package openai

import (
	"strings"
	"testing"
)

func TestParseErrorMessageIncludesTypeCodeAndParam(t *testing.T) {
	body := []byte(`{"error":{"message":"Unsupported parameter: 'temperature' is not supported with this model.","type":"openai_error","param":"temperature","code":"unsupported_parameter"}}`)
	got := parseErrorMessage(body)
	for _, want := range []string{
		"Unsupported parameter",
		"type=openai_error",
		"code=unsupported_parameter",
		"param=temperature",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("message = %q, want containing %q", got, want)
		}
	}
}
