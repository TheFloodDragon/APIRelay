package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/apirelay/apirelay/dto"
)

func TestParseOpenAIAdvancedFieldsAndUsage(t *testing.T) {
	ir, err := ParseOpenAIRequest([]byte(`{"model":"gpt","messages":[],"tool_choice":{"type":"function","function":{"name":"lookup"}},"parallel_tool_calls":false,"response_format":{"type":"json_schema","json_schema":{"name":"answer","strict":true,"schema":{"type":"object"}}},"reasoning_effort":"high","top_k":20}`))
	if err != nil {
		t.Fatal(err)
	}
	if ir.ToolChoice == nil || ir.ToolChoice.Mode != "tool" || ir.ToolChoice.Name != "lookup" || !ir.ToolChoice.DisableParallel {
		t.Fatalf("tool choice lost: %+v", ir.ToolChoice)
	}
	if ir.ResponseFormat == nil || ir.ResponseFormat.Name != "answer" || ir.Thinking == nil || ir.Thinking.Effort != "high" || ir.TopK == nil || *ir.TopK != 20 {
		t.Fatalf("advanced fields lost: %+v", ir)
	}
	usage := openAIUsageToIR(&dto.OpenAIUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14, PromptTokensDetails: &dto.OpenAIPromptTokenDetails{CachedTokens: 6}, CompletionTokensDetails: &dto.OpenAICompletionTokenDetails{ReasoningTokens: 3}})
	if usage.CacheReadInputTokens != 6 || usage.ReasoningTokens != 3 {
		t.Fatalf("usage details lost: %+v", usage)
	}
}

func TestAnthropicThinkingRoundTripAndCacheUsage(t *testing.T) {
	ir, err := ParseAnthropicRequest([]byte(`{"model":"claude","max_tokens":100,"top_k":12,"thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"work","signature":"sig"},{"type":"text","text":"answer"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if ir.Thinking == nil || ir.Thinking.BudgetTokens != 1024 || len(ir.Messages) != 1 || len(ir.Messages[0].Parts) < 2 || ir.Messages[0].Parts[0].Signature != "sig" {
		t.Fatalf("thinking fields lost: %+v", ir)
	}
	built := BuildAnthropicRequest(ir, "mapped")
	var blocks []dto.AnthropicContentBlock
	if err := json.Unmarshal(built.Messages[0].Content, &blocks); err != nil || len(blocks) < 2 || blocks[0].Signature != "sig" {
		t.Fatalf("thinking round trip failed: %v %+v", err, blocks)
	}
	usage := anthropicUsageToIR(&dto.AnthropicUsage{InputTokens: 5, OutputTokens: 2, CacheCreationInputTokens: 3, CacheReadInputTokens: 7})
	if usage.PromptTokens != 15 || usage.CacheCreationInputTokens != 3 || usage.CacheReadInputTokens != 7 {
		t.Fatalf("anthropic cache usage lost: %+v", usage)
	}
}

func TestResponsesAdvancedFieldsAndUsage(t *testing.T) {
	ir, err := ParseResponsesRequest([]byte(`{"model":"gpt","input":"hi","tool_choice":"required","parallel_tool_calls":false,"text":{"format":{"type":"json_schema","name":"answer","strict":true,"schema":{"type":"object"}}},"reasoning":{"effort":"medium","summary":"auto"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if ir.ToolChoice == nil || ir.ToolChoice.Mode != "required" || !ir.ToolChoice.DisableParallel || ir.ResponseFormat == nil || ir.ResponseFormat.Name != "answer" || ir.Thinking == nil || ir.Thinking.Summary != "auto" {
		t.Fatalf("responses advanced fields lost: %+v", ir)
	}
	usage := responsesUsageToIR(&dto.ResponsesUsage{InputTokens: 9, OutputTokens: 5, TotalTokens: 14, InputTokensDetails: &dto.ResponsesInputTokenDetails{CachedTokens: 4}, OutputTokensDetails: &dto.ResponsesOutputTokenDetails{ReasoningTokens: 2}})
	if usage.CacheReadInputTokens != 4 || usage.ReasoningTokens != 2 {
		t.Fatalf("responses usage details lost: %+v", usage)
	}
}
