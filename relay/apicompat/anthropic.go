package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apirelay/apirelay/dto"
)

// ============================================================================
// Anthropic Messages -> IR （入站解析）
// ============================================================================

// ParseAnthropicRequest 将 Anthropic 请求体解析为统一 IR。
func ParseAnthropicRequest(body []byte) (*dto.UnifiedRequest, error) {
	var req dto.AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	if req.Model == "" {
		return nil, fmt.Errorf("missing model")
	}

	ir := &dto.UnifiedRequest{
		Model:          req.Model,
		Stream:         req.Stream,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		TopK:           req.TopK,
		Stop:           req.StopSequences,
		SourceEndpoint: "anthropic",
		Raw:            body,
	}
	if req.ToolChoice != nil {
		mode := req.ToolChoice.Type
		if mode == "any" {
			mode = "required"
		}
		ir.ToolChoice = &dto.UnifiedToolChoice{Mode: mode, Name: req.ToolChoice.Name, DisableParallel: req.ToolChoice.DisableParallelToolUse}
	}
	if req.Thinking != nil {
		ir.Thinking = &dto.UnifiedThinkingConfig{Type: req.Thinking.Type, BudgetTokens: req.Thinking.BudgetTokens}
	}
	if containsJSONKey(body, "cache_control") {
		ir.UnsupportedFeatures = append(ir.UnsupportedFeatures, "cache_control")
	}
	if req.MaxTokens > 0 {
		mt := req.MaxTokens
		ir.MaxTokens = &mt
	}
	ir.System = parseAnthropicSystem(req.System)

	for _, m := range req.Messages {
		um := dto.UnifiedMessage{Role: dto.UnifiedRole(m.Role)}
		blocks, isText := parseAnthropicContent(m.Content)
		if isText {
			um.Content = blocks.text
			ir.Messages = append(ir.Messages, um)
			continue
		}
		// 处理 content block 数组
		um.Content = blocks.text
		um.Parts = blocks.parts
		um.ToolCalls = blocks.toolCalls
		// tool_result 块单独拆成 tool 角色消息（OpenAI 风格）
		for _, tr := range blocks.toolResults {
			ir.Messages = append(ir.Messages, dto.UnifiedMessage{
				Role:       dto.RoleTool,
				ToolCallID: tr.id,
				Content:    tr.content,
			})
		}
		if um.Content != "" || len(um.Parts) > 0 || len(um.ToolCalls) > 0 {
			ir.Messages = append(ir.Messages, um)
		}
	}

	for _, t := range req.Tools {
		ir.Tools = append(ir.Tools, dto.UnifiedTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return ir, nil
}

func parseAnthropicSystem(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []dto.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}
	return ""
}

type anthropicToolResult struct {
	id      string
	content string
}

type anthropicParsedContent struct {
	text        string
	parts       []dto.UnifiedContentPart
	toolCalls   []dto.UnifiedToolCall
	toolResults []anthropicToolResult
}

// parseAnthropicContent 解析消息 content。返回 (解析结果, 是否为纯字符串)。
func parseAnthropicContent(raw json.RawMessage) (anthropicParsedContent, bool) {
	var out anthropicParsedContent
	if len(raw) == 0 {
		return out, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		out.text = s
		return out, true
	}
	var blocks []dto.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return out, true
	}
	var sb strings.Builder
	for _, b := range blocks {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
			out.parts = append(out.parts, dto.UnifiedContentPart{Type: "text", Text: b.Text})
		case "image":
			if b.Source != nil {
				url := b.Source.URL
				if url == "" && b.Source.Data != "" {
					url = fmt.Sprintf("data:%s;base64,%s", b.Source.MediaType, b.Source.Data)
				}
				out.parts = append(out.parts, dto.UnifiedContentPart{Type: "image_url", ImageURL: url})
			}
		case "thinking":
			out.parts = append(out.parts, dto.UnifiedContentPart{Type: "thinking", Thinking: b.Thinking, Signature: b.Signature})
		case "redacted_thinking":
			out.parts = append(out.parts, dto.UnifiedContentPart{Type: "redacted_thinking", Data: b.Data})
		case "tool_use":
			out.toolCalls = append(out.toolCalls, dto.UnifiedToolCall{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: string(b.Input),
			})
		case "tool_result":
			out.toolResults = append(out.toolResults, anthropicToolResult{
				id:      b.ToolUseID,
				content: rawContentToText(b.Content),
			})
		}
	}
	out.text = sb.String()
	return out, false
}

func rawContentToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []dto.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
		return sb.String()
	}
	return string(raw)
}

// ============================================================================
// IR -> Anthropic Messages 请求（供 Anthropic 上游适配器使用）
// ============================================================================

// BuildAnthropicRequest 将 IR 转换为 Anthropic 请求体。
func BuildAnthropicRequest(ir *dto.UnifiedRequest, upstreamModel string) *dto.AnthropicRequest {
	req := &dto.AnthropicRequest{
		Model:         upstreamModel,
		Stream:        ir.Stream,
		Temperature:   ir.Temperature,
		TopP:          ir.TopP,
		TopK:          ir.TopK,
		StopSequences: ir.Stop,
	}
	if choice := ir.ToolChoice; choice != nil {
		mode := choice.Mode
		if mode == "required" {
			mode = "any"
		}
		req.ToolChoice = &dto.AnthropicToolChoice{Type: mode, Name: choice.Name, DisableParallelToolUse: choice.DisableParallel}
	}
	if thinking := ir.Thinking; thinking != nil && (thinking.Type != "" || thinking.BudgetTokens > 0) {
		req.Thinking = &dto.AnthropicThinking{Type: thinking.Type, BudgetTokens: thinking.BudgetTokens}
	}
	// Anthropic 强制要求 max_tokens
	if ir.MaxTokens != nil && *ir.MaxTokens > 0 {
		req.MaxTokens = *ir.MaxTokens
	} else {
		req.MaxTokens = 4096
	}
	if ir.System != "" {
		sysRaw, _ := json.Marshal(ir.System)
		req.System = sysRaw
	}

	for _, m := range ir.Messages {
		switch m.Role {
		case dto.RoleTool:
			// tool 结果 -> user 消息内的 tool_result 块
			block := dto.AnthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
			}
			block.Content, _ = json.Marshal(m.Content)
			content, _ := json.Marshal([]dto.AnthropicContentBlock{block})
			req.Messages = append(req.Messages, dto.AnthropicMessage{Role: "user", Content: content})
		case dto.RoleAssistant:
			blocks := []dto.AnthropicContentBlock{}
			for _, part := range m.Parts {
				switch part.Type {
				case "thinking":
					blocks = append(blocks, dto.AnthropicContentBlock{Type: "thinking", Thinking: part.Thinking, Signature: part.Signature})
				case "redacted_thinking":
					blocks = append(blocks, dto.AnthropicContentBlock{Type: "redacted_thinking", Data: part.Data})
				}
			}
			if m.Content != "" {
				blocks = append(blocks, dto.AnthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				input := json.RawMessage(tc.Arguments)
				if len(input) == 0 {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, dto.AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			content, _ := json.Marshal(blocks)
			req.Messages = append(req.Messages, dto.AnthropicMessage{Role: "assistant", Content: content})
		default: // user / system(已抽到 ir.System)
			role := string(m.Role)
			if role == "system" {
				// 兜底：未被抽走的 system 并入 user
				role = "user"
			}
			content := buildAnthropicUserContent(m)
			req.Messages = append(req.Messages, dto.AnthropicMessage{Role: role, Content: content})
		}
	}

	for _, t := range ir.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		req.Tools = append(req.Tools, dto.AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return req
}

func buildAnthropicUserContent(m dto.UnifiedMessage) json.RawMessage {
	if len(m.Parts) == 0 {
		c, _ := json.Marshal(m.Content)
		return c
	}
	var blocks []dto.AnthropicContentBlock
	for _, p := range m.Parts {
		switch p.Type {
		case "text":
			blocks = append(blocks, dto.AnthropicContentBlock{Type: "text", Text: p.Text})
		case "image_url":
			blocks = append(blocks, dto.AnthropicContentBlock{
				Type:   "image",
				Source: &dto.AnthropicImageSource{Type: "url", URL: p.ImageURL},
			})
		}
	}
	c, _ := json.Marshal(blocks)
	return c
}

// ============================================================================
// Anthropic 响应 -> IR
// ============================================================================

// AnthropicResponseToIR 将 Anthropic 非流式响应转为统一响应。
func AnthropicResponseToIR(resp *dto.AnthropicResponse) *dto.UnifiedResponse {
	out := &dto.UnifiedResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		FinishReason: mapAnthropicStopReason(resp.StopReason),
	}
	var sb strings.Builder
	for _, b := range resp.Content {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, dto.UnifiedToolCall{
				ID:        b.ID,
				Name:      b.Name,
				Arguments: string(b.Input),
			})
		}
	}
	out.Content = sb.String()
	out.Usage = anthropicUsageToIR(&resp.Usage)
	return out
}

func anthropicUsageToIR(usage *dto.AnthropicUsage) dto.Usage {
	if usage == nil {
		return dto.Usage{}
	}
	promptTotal := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	return dto.Usage{
		PromptTokens:             promptTotal,
		CompletionTokens:         usage.OutputTokens,
		TotalTokens:              promptTotal + usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
	}
}

func mapAnthropicStopReason(r string) string {
	switch r {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return r
	}
}
