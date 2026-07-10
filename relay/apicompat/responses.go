package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// OpenAI Responses -> IR （入站解析）
// ============================================================================

// ParseResponsesRequest 将 Responses 请求体解析为统一 IR。
func ParseResponsesRequest(body []byte) (*dto.UnifiedRequest, error) {
	var req dto.ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse responses request: %w", err)
	}
	if req.Model == "" {
		return nil, fmt.Errorf("missing model")
	}

	ir := &dto.UnifiedRequest{
		Model:          req.Model,
		System:         req.Instructions,
		Stream:         req.Stream,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		MaxTokens:      req.MaxOutputTokens,
		SourceEndpoint: "responses",
		Raw:            body,
	}

	ir.Messages = parseResponsesInput(req.Input)

	for _, t := range req.Tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		ir.Tools = append(ir.Tools, dto.UnifiedTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return ir, nil
}

// parseResponsesInput 解析 input（string 或 item 数组）为统一消息。
func parseResponsesInput(raw json.RawMessage) []dto.UnifiedMessage {
	if len(raw) == 0 {
		return nil
	}
	// 字符串 input 直接作为 user 消息
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []dto.UnifiedMessage{{Role: dto.RoleUser, Content: s}}
	}

	var items []dto.ResponsesInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}

	var msgs []dto.UnifiedMessage
	for _, it := range items {
		switch it.Type {
		case "function_call":
			msgs = append(msgs, dto.UnifiedMessage{
				Role: dto.RoleAssistant,
				ToolCalls: []dto.UnifiedToolCall{{
					ID:        it.CallID,
					Name:      it.Name,
					Arguments: it.Arguments,
				}},
			})
		case "function_call_output":
			msgs = append(msgs, dto.UnifiedMessage{
				Role:       dto.RoleTool,
				ToolCallID: it.CallID,
				Content:    it.Output,
			})
		default: // message
			um := dto.UnifiedMessage{Role: dto.UnifiedRole(orDefault(it.Role, "user"))}
			um.Content, um.Parts = parseResponsesContent(it.Content)
			msgs = append(msgs, um)
		}
	}
	return msgs
}

func parseResponsesContent(raw json.RawMessage) (string, []dto.UnifiedContentPart) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var parts []dto.ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", nil
	}
	var sb strings.Builder
	var out []dto.UnifiedContentPart
	for _, p := range parts {
		switch p.Type {
		case "input_text", "output_text", "text":
			sb.WriteString(p.Text)
			out = append(out, dto.UnifiedContentPart{Type: "text", Text: p.Text})
		case "input_image":
			out = append(out, dto.UnifiedContentPart{Type: "image_url", ImageURL: p.ImageURL})
		}
	}
	return sb.String(), out
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ============================================================================
// IR -> Responses 请求（供 Responses 上游适配器使用）
// ============================================================================

// BuildResponsesRequest 将 IR 转换为 Responses 请求体。
func BuildResponsesRequest(ir *dto.UnifiedRequest, upstreamModel string) *dto.ResponsesRequest {
	req := &dto.ResponsesRequest{
		Model:           upstreamModel,
		Instructions:    ir.System,
		Stream:          ir.Stream,
		Temperature:     ir.Temperature,
		TopP:            ir.TopP,
		MaxOutputTokens: ir.MaxTokens,
	}

	var items []dto.ResponsesInputItem
	for _, m := range ir.Messages {
		switch m.Role {
		case dto.RoleTool:
			items = append(items, dto.ResponsesInputItem{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: m.Content,
			})
		case dto.RoleAssistant:
			if m.Content != "" {
				items = append(items, dto.ResponsesInputItem{
					Type:    "message",
					Role:    "assistant",
					Content: responsesMessageContent("output_text", m),
				})
			}
			for _, tc := range m.ToolCalls {
				items = append(items, dto.ResponsesInputItem{
					Type:      "function_call",
					CallID:    tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
		default:
			items = append(items, dto.ResponsesInputItem{
				Type:    "message",
				Role:    orDefault(string(m.Role), "user"),
				Content: responsesMessageContent("input_text", m),
			})
		}
	}
	inputRaw, _ := json.Marshal(items)
	req.Input = inputRaw

	for _, t := range ir.Tools {
		req.Tools = append(req.Tools, dto.ResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return req
}

func responsesTextContent(typ, text string) json.RawMessage {
	parts := []dto.ResponsesContentPart{{Type: typ, Text: text}}
	b, _ := json.Marshal(parts)
	return b
}

// responsesMessageContent 构造 Responses 消息 content 数组，支持多模态图片（A6）。
// textType 为该角色的文本块类型（用户 input_text / 助手 output_text）。
// 无 parts 时回退为单个文本块。
func responsesMessageContent(textType string, m dto.UnifiedMessage) json.RawMessage {
	if len(m.Parts) == 0 {
		return responsesTextContent(textType, m.Content)
	}
	parts := make([]dto.ResponsesContentPart, 0, len(m.Parts))
	for _, p := range m.Parts {
		switch p.Type {
		case "text":
			parts = append(parts, dto.ResponsesContentPart{Type: textType, Text: p.Text})
		case "image_url":
			if p.ImageURL != "" {
				parts = append(parts, dto.ResponsesContentPart{Type: "input_image", ImageURL: p.ImageURL})
			}
		}
	}
	if len(parts) == 0 {
		return responsesTextContent(textType, m.Content)
	}
	b, _ := json.Marshal(parts)
	return b
}

// ============================================================================
// Responses 响应 -> IR
// ============================================================================

// ResponsesResponseToIR 将 Responses 非流式响应转为统一响应。
func ResponsesResponseToIR(resp *dto.ResponsesResponse) *dto.UnifiedResponse {
	out := &dto.UnifiedResponse{ID: resp.ID, Model: resp.Model, FinishReason: "stop"}
	var sb strings.Builder
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, p := range item.Content {
				if p.Type == "output_text" || p.Type == "text" {
					sb.WriteString(p.Text)
				}
			}
		case "function_call":
			out.ToolCalls = append(out.ToolCalls, dto.UnifiedToolCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: item.Arguments,
			})
			out.FinishReason = "tool_calls"
		}
	}
	out.Content = sb.String()
	if resp.Usage != nil {
		out.Usage = dto.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return out
}

// ParseResponsesStreamEvent 解析一条 Responses SSE data（JSON）为统一增量。
// 返回 nil chunk 表示该事件无需向下游输出。
func ParseResponsesStreamEvent(data []byte) (*dto.UnifiedStreamChunk, error) {
	var ev dto.ResponsesStreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, err
	}
	switch ev.Type {
	case "response.output_text.delta":
		return &dto.UnifiedStreamChunk{DeltaText: ev.Delta}, nil
	case "response.function_call_arguments.delta":
		idx := ev.OutputIndex
		return &dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{{Arguments: ev.Delta, Index: &idx}}}, nil
	case "response.output_item.added":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			idx := ev.OutputIndex
			return &dto.UnifiedStreamChunk{ToolCalls: []dto.UnifiedToolCall{{
				ID:    ev.Item.CallID,
				Name:  ev.Item.Name,
				Index: &idx,
			}}}, nil
		}
		return nil, nil
	case "response.completed":
		chunk := &dto.UnifiedStreamChunk{FinishReason: "stop", Done: true}
		if ev.Response != nil && ev.Response.Usage != nil {
			chunk.Usage = &dto.Usage{
				PromptTokens:     ev.Response.Usage.InputTokens,
				CompletionTokens: ev.Response.Usage.OutputTokens,
				TotalTokens:      ev.Response.Usage.TotalTokens,
			}
		}
		return chunk, nil
	default:
		return nil, nil
	}
}
