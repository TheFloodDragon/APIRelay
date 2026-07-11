package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
)

// CompatibilityError 表示请求特性无法无损转换到目标协议。
type CompatibilityError struct {
	Target   constant.APIType
	Features []string
}

func (e *CompatibilityError) Error() string {
	return fmt.Sprintf("目标协议 %s 无法无损表达：%s", apiTypeName(e.Target), strings.Join(e.Features, "、"))
}

// ValidateCompatibility 校验跨协议转换；同协议 Raw 透传始终允许。
func ValidateCompatibility(ep constant.EndpointType, target constant.APIType, ir *dto.UnifiedRequest) error {
	if ir == nil || SameProtocol(ep, target) {
		return nil
	}
	features := append([]string(nil), ir.UnsupportedFeatures...)
	if ir.ResponseFormat != nil && ir.ResponseFormat.Type != "" && target == constant.APITypeAnthropic {
		features = append(features, "response_format")
	}
	if ir.TopK != nil && target != constant.APITypeAnthropic {
		features = append(features, "top_k")
	}
	if t := ir.Thinking; t != nil {
		if (t.Type != "" || t.BudgetTokens > 0) && target != constant.APITypeAnthropic {
			features = append(features, "thinking.budget_tokens")
		}
		if t.Effort != "" && target == constant.APITypeAnthropic {
			features = append(features, "reasoning.effort")
		}
		if t.Summary != "" && target != constant.APITypeResponses {
			features = append(features, "reasoning.summary")
		}
	}
	if target != constant.APITypeAnthropic {
		for _, message := range ir.Messages {
			for _, part := range message.Parts {
				if part.Type == "thinking" || part.Type == "redacted_thinking" {
					features = append(features, "messages[].content."+part.Type)
				}
			}
		}
	}
	features = uniqueStrings(features)
	if len(features) == 0 {
		return nil
	}
	return &CompatibilityError{Target: target, Features: features}
}

func apiTypeName(t constant.APIType) string {
	switch t {
	case constant.APITypeAnthropic:
		return "Anthropic"
	case constant.APITypeResponses:
		return "OpenAI Responses"
	default:
		return "OpenAI Chat Completions"
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func containsJSONKey(body []byte, key string) bool {
	var value any
	if json.Unmarshal(body, &value) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			if _, ok := typed[key]; ok {
				return true
			}
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}

func parseToolChoice(raw json.RawMessage, disableParallel bool) *dto.UnifiedToolChoice {
	if len(raw) == 0 {
		if disableParallel {
			return &dto.UnifiedToolChoice{Mode: "auto", DisableParallel: true}
		}
		return nil
	}
	var mode string
	if json.Unmarshal(raw, &mode) == nil {
		return &dto.UnifiedToolChoice{Mode: mode, DisableParallel: disableParallel}
	}
	var object struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	name := object.Name
	if name == "" {
		name = object.Function.Name
	}
	mode = object.Type
	if mode == "function" {
		mode = "tool"
	}
	return &dto.UnifiedToolChoice{Mode: mode, Name: name, DisableParallel: disableParallel}
}

func openAIToolChoice(choice *dto.UnifiedToolChoice) (json.RawMessage, *bool) {
	if choice == nil {
		return nil, nil
	}
	parallel := true
	if choice.DisableParallel {
		parallel = false
	}
	if choice.Mode == "tool" {
		value, _ := json.Marshal(map[string]any{"type": "function", "function": map[string]string{"name": choice.Name}})
		return value, &parallel
	}
	value, _ := json.Marshal(choice.Mode)
	return value, &parallel
}

func responsesToolChoice(choice *dto.UnifiedToolChoice) (json.RawMessage, *bool) {
	if choice == nil {
		return nil, nil
	}
	parallel := !choice.DisableParallel
	if choice.Mode == "tool" {
		value, _ := json.Marshal(map[string]string{"type": "function", "name": choice.Name})
		return value, &parallel
	}
	value, _ := json.Marshal(choice.Mode)
	return value, &parallel
}
