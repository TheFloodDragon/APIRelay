package apicompat

import (
	"encoding/json"
	"fmt"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// OpenAI Chat Completions <-> IR 互转
// ============================================================================

// ParseOpenAIRequest 将 OpenAI 请求体解析为统一 IR。
func ParseOpenAIRequest(body []byte) (*dto.UnifiedRequest, error) {
	var req dto.OpenAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}
	if req.Model == "" {
		return nil, fmt.Errorf("missing model")
	}

	ir := &dto.UnifiedRequest{
		Model:          req.Model,
		MaxTokens:      req.MaxTokens,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		Stream:         req.Stream,
		Stop:           []string(req.Stop),
		SourceEndpoint: "openai",
		Raw:            body,
	}

	for _, m := range req.Messages {
		um := dto.UnifiedMessage{
			Role:       dto.UnifiedRole(m.Role),
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}
		um.Content, um.Parts = parseOpenAIContent(m.Content)
		for _, tc := range m.ToolCalls {
			um.ToolCalls = append(um.ToolCalls, dto.UnifiedToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		// system 消息归并到 IR.System，便于转 Anthropic
		if m.Role == "system" && um.Content != "" {
			if ir.System != "" {
				ir.System += "\n"
			}
			ir.System += um.Content
			continue
		}
		ir.Messages = append(ir.Messages, um)
	}

	for _, t := range req.Tools {
		ir.Tools = append(ir.Tools, dto.UnifiedTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	return ir, nil
}

// parseOpenAIContent 解析 content（可能是字符串或多模态数组）。
func parseOpenAIContent(raw json.RawMessage) (string, []dto.UnifiedContentPart) {
	if len(raw) == 0 {
		return "", nil
	}
	// 尝试字符串
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	// 尝试数组
	var arr []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var parts []dto.UnifiedContentPart
		var textAccum string
		for _, p := range arr {
			switch p.Type {
			case "text":
				textAccum += p.Text
				parts = append(parts, dto.UnifiedContentPart{Type: "text", Text: p.Text})
			case "image_url":
				parts = append(parts, dto.UnifiedContentPart{Type: "image_url", ImageURL: p.ImageURL.URL})
			}
		}
		return textAccum, parts
	}
	return "", nil
}

// BuildOpenAIRequest 将 IR 转换为 OpenAI 请求体（供 OpenAI 上游适配器使用）。
func BuildOpenAIRequest(ir *dto.UnifiedRequest, upstreamModel string) *dto.OpenAIChatRequest {
	req := &dto.OpenAIChatRequest{
		Model:       upstreamModel,
		MaxTokens:   ir.MaxTokens,
		Temperature: ir.Temperature,
		TopP:        ir.TopP,
		Stream:      ir.Stream,
		Stop:        dto.StopSequences(ir.Stop),
	}
	if ir.Stream {
		req.StreamOptions = &dto.OpenAIStreamOptions{IncludeUsage: true}
	}
	// system 优先作为首条 system 消息
	if ir.System != "" {
		c, _ := json.Marshal(ir.System)
		req.Messages = append(req.Messages, dto.OpenAIMessage{Role: "system", Content: c})
	}
	for _, m := range ir.Messages {
		om := dto.OpenAIMessage{
			Role:       string(m.Role),
			Name:       m.Name,
			ToolCallID: m.ToolCallID,
		}
		if m.Content != "" {
			c, _ := json.Marshal(m.Content)
			om.Content = c
		}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, dto.OpenAIToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: dto.OpenAIToolCallFunc{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		req.Messages = append(req.Messages, om)
	}
	for _, t := range ir.Tools {
		req.Tools = append(req.Tools, dto.OpenAITool{
			Type: "function",
			Function: dto.OpenAIToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return req
}

// OpenAIResponseToIR 将 OpenAI 非流式响应转为统一响应。
func OpenAIResponseToIR(resp *dto.OpenAIChatResponse) *dto.UnifiedResponse {
	out := &dto.UnifiedResponse{
		ID:    resp.ID,
		Model: resp.Model,
	}
	if len(resp.Choices) > 0 {
		ch := resp.Choices[0]
		if ch.Message != nil {
			var s string
			_ = json.Unmarshal(ch.Message.Content, &s)
			out.Content = s
			for _, tc := range ch.Message.ToolCalls {
				out.ToolCalls = append(out.ToolCalls, dto.UnifiedToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
		if ch.FinishReason != nil {
			out.FinishReason = *ch.FinishReason
		}
	}
	if resp.Usage != nil {
		out.Usage = dto.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return out
}

// ParseOpenAIStreamChunk 解析一条 OpenAI SSE data 行（不含 "data: " 前缀）为统一增量。
func ParseOpenAIStreamChunk(data []byte) (*dto.UnifiedStreamChunk, error) {
	var chunk dto.OpenAIChatResponse
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}
	out := &dto.UnifiedStreamChunk{}
	if len(chunk.Choices) > 0 {
		ch := chunk.Choices[0]
		if ch.Delta != nil {
			out.DeltaText = ch.Delta.Content
			for _, tc := range ch.Delta.ToolCalls {
				out.ToolCalls = append(out.ToolCalls, dto.UnifiedToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				})
			}
		}
		if ch.FinishReason != nil {
			out.FinishReason = *ch.FinishReason
		}
	}
	if chunk.Usage != nil {
		out.Usage = &dto.Usage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}
	return out, nil
}
