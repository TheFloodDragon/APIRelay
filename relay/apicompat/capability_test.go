package apicompat

import (
	"strings"
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
)

func TestValidateCompatibilityMatrix(t *testing.T) {
	topK := 40
	ir := &dto.UnifiedRequest{
		TopK:                &topK,
		ResponseFormat:      &dto.UnifiedResponseFormat{Type: "json_schema"},
		UnsupportedFeatures: []string{"cache_control"},
	}
	if err := ValidateCompatibility(constant.EndpointOpenAI, constant.APITypeOpenAI, ir); err != nil {
		t.Fatalf("same protocol must preserve raw fields: %v", err)
	}
	err := ValidateCompatibility(constant.EndpointOpenAI, constant.APITypeAnthropic, ir)
	if err == nil || !strings.Contains(err.Error(), "response_format") || !strings.Contains(err.Error(), "cache_control") {
		t.Fatalf("unexpected Anthropic compatibility result: %v", err)
	}
	err = ValidateCompatibility(constant.EndpointAnthropic, constant.APITypeResponses, &dto.UnifiedRequest{TopK: &topK})
	if err == nil || !strings.Contains(err.Error(), "top_k") {
		t.Fatalf("Responses should reject top_k: %v", err)
	}
}

func TestContainsJSONKeyIgnoresStringContent(t *testing.T) {
	if containsJSONKey([]byte(`{"message":"mentions cache_control only"}`), "cache_control") {
		t.Fatal("plain string content must not be treated as a cache_control field")
	}
	if !containsJSONKey([]byte(`{"messages":[{"content":[{"type":"text","cache_control":{"type":"ephemeral"}}]}]}`), "cache_control") {
		t.Fatal("nested cache_control field should be detected")
	}
}

func TestValidateCompatibilityAllowsPortableToolChoice(t *testing.T) {
	ir := &dto.UnifiedRequest{ToolChoice: &dto.UnifiedToolChoice{Mode: "tool", Name: "lookup", DisableParallel: true}}
	for _, target := range []constant.APIType{constant.APITypeOpenAI, constant.APITypeAnthropic, constant.APITypeResponses} {
		if err := ValidateCompatibility(constant.EndpointOpenAI, target, ir); err != nil {
			t.Fatalf("target %v rejected portable tool choice: %v", target, err)
		}
	}
}
