package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

type anthropicMessagesRequest struct {
	Model         string                   `json:"model"`
	Messages      []anthropicMessage       `json:"messages"`
	MaxTokens     int                      `json:"max_tokens"`
	Temperature   *float64                 `json:"temperature,omitempty"`
	TopP          *float64                 `json:"top_p,omitempty"`
	Stream        bool                     `json:"stream,omitempty"`
	StopSequences []string                 `json:"stop_sequences,omitempty"`
	System        interface{}              `json:"system,omitempty"`
	Tools         []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice    interface{}              `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicMessagesResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason,omitempty"`
	Usage      *anthropicUsage    `json:"usage,omitempty"`
}

type anthropicContent struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamChunk struct {
	Type         string                     `json:"type"`
	Index        int                        `json:"index,omitempty"`
	Delta        *anthropicDelta            `json:"delta,omitempty"`
	ContentBlock *anthropicContent          `json:"content_block,omitempty"`
	Message      *anthropicMessagesResponse `json:"message,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`

	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// AnthropicMessagesRequestToProtocol 将 Anthropic Messages 请求转为通用文本聊天请求。
func AnthropicMessagesRequestToProtocol(body []byte) (*ChatRequest, error) {
	var req anthropicMessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 请求失败: %w", err)
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		System:      contentToText(req.System),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
		Tools:       anthropicToolsToOpenAI(req.Tools),
		ToolChoice:  anthropicToolChoiceToOpenAI(req.ToolChoice),
	}
	if req.MaxTokens > 0 {
		chatReq.MaxTokens = &req.MaxTokens
	}
	for _, message := range req.Messages {
		role := normalizeRole(message.Role)
		if role == "system" {
			role = "user"
		}
		for _, chatMessage := range anthropicMessageToChatMessages(role, message.Content) {
			chatReq.Messages = append(chatReq.Messages, chatMessage)
		}
	}
	if len(chatReq.Messages) == 0 {
		return nil, fmt.Errorf("缺少可转发的 messages")
	}
	return chatReq, nil
}

// ProtocolToAnthropicMessagesRequest 将通用文本聊天请求转为 Anthropic Messages 请求。
func ProtocolToAnthropicMessagesRequest(req *ChatRequest) ([]byte, error) {
	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	anthropicReq := anthropicMessagesRequest{
		Model:         req.Model,
		MaxTokens:     maxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		Stream:        req.Stream,
		StopSequences: req.Stop,
		Tools:         openAIToolsToAnthropic(req.Tools),
		ToolChoice:    openAIToolChoiceToAnthropic(req.ToolChoice),
	}
	if req.System != "" {
		anthropicReq.System = req.System
	}
	for _, message := range req.Messages {
		anthropicMessage, ok := chatMessageToAnthropicMessage(message)
		if ok {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage)
		}
	}
	if len(anthropicReq.Messages) == 0 && req.System != "" {
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{Role: "user", Content: req.System})
		anthropicReq.System = nil
	}
	return json.Marshal(anthropicReq)
}

// AnthropicMessagesResponseToProtocol 将 Anthropic Messages 响应转为通用响应。
func AnthropicMessagesResponseToProtocol(body []byte) (*ChatResponse, error) {
	var resp anthropicMessagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}

	parts := make([]string, 0, len(resp.Content))
	toolCalls := make([]map[string]interface{}, 0)
	for _, item := range resp.Content {
		if item.Type == "text" && item.Text != "" {
			parts = append(parts, item.Text)
		}
		if item.Type == "tool_use" {
			toolCalls = append(toolCalls, anthropicToolUseToOpenAI(item))
		}
	}
	chatResp := &ChatResponse{
		ID:           firstNonEmpty(resp.ID, generatedID("msg")),
		Model:        resp.Model,
		Role:         firstNonEmpty(resp.Role, "assistant"),
		Content:      strings.Join(parts, ""),
		ToolCalls:    toolCalls,
		FinishReason: normalizeFinishReason(resp.StopReason),
		Created:      nowUnix(),
	}
	if resp.Usage != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	return chatResp, nil
}

// ProtocolToAnthropicMessagesResponse 将通用响应转为 Anthropic Messages 响应。
func ProtocolToAnthropicMessagesResponse(resp *ChatResponse) ([]byte, error) {
	content := make([]anthropicContent, 0, 1+len(resp.ToolCalls))
	if resp.Content != "" || len(resp.ToolCalls) == 0 {
		content = append(content, anthropicContent{Type: "text", Text: resp.Content})
	}
	for _, toolCall := range resp.ToolCalls {
		content = append(content, openAIToolCallToAnthropic(toolCall))
	}
	anthropicResp := anthropicMessagesResponse{
		ID:         firstNonEmpty(resp.ID, generatedID("msg")),
		Type:       "message",
		Role:       firstNonEmpty(resp.Role, "assistant"),
		Model:      resp.Model,
		StopReason: finishReasonToAnthropic(resp.FinishReason),
		Content:    content,
	}
	if anthropicResp.Role == "model" {
		anthropicResp.Role = "assistant"
	}
	if resp.Usage != nil {
		anthropicResp.Usage = &anthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		}
	}
	return json.Marshal(anthropicResp)
}

// AnthropicStreamEventsFromData 将单条 Anthropic SSE data 负载转为通用流事件。
func AnthropicStreamEventsFromData(data string) ([]StreamEvent, error) {
	if data == "[DONE]" {
		return []StreamEvent{{Done: true}}, nil
	}
	var chunk anthropicStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, err
	}

	switch chunk.Type {
	case "message_start":
		event := StreamEvent{Start: true, Role: "assistant"}
		if chunk.Message != nil {
			event.ID = chunk.Message.ID
			event.Model = chunk.Message.Model
		}
		return []StreamEvent{event}, nil
	case "content_block_start":
		if chunk.ContentBlock != nil && chunk.ContentBlock.Type == "tool_use" {
			toolCall := anthropicToolUseToOpenAI(*chunk.ContentBlock)
			name, arguments := toolCallNameAndArguments(toolCall)
			return []StreamEvent{{ToolCalls: []map[string]interface{}{toolCall}, ToolCallID: toolCallID(toolCall), ToolName: name, ToolArguments: arguments, Index: chunk.Index}}, nil
		}
		if chunk.ContentBlock != nil && chunk.ContentBlock.Text != "" {
			return []StreamEvent{{Content: chunk.ContentBlock.Text, Index: chunk.Index}}, nil
		}
	case "content_block_delta":
		if chunk.Delta != nil && chunk.Delta.PartialJSON != "" {
			toolCall := map[string]interface{}{
				"index": chunk.Index,
				"type":  "function",
				"function": map[string]interface{}{
					"arguments": chunk.Delta.PartialJSON,
				},
			}
			return []StreamEvent{{ToolCalls: []map[string]interface{}{toolCall}, ToolArguments: chunk.Delta.PartialJSON, Index: chunk.Index}}, nil
		}
		if chunk.Delta != nil && chunk.Delta.Text != "" {
			return []StreamEvent{{Content: chunk.Delta.Text, Index: chunk.Index}}, nil
		}
	case "message_delta":
		if chunk.Delta != nil && chunk.Delta.StopReason != "" {
			return []StreamEvent{{FinishReason: normalizeFinishReason(chunk.Delta.StopReason)}}, nil
		}
	case "message_stop":
		return []StreamEvent{{Done: true}}, nil
	}
	return nil, nil
}

func anthropicMessageToChatMessages(role string, content interface{}) []ChatMessage {
	blocks, ok := content.([]interface{})
	if !ok {
		text := contentToText(content)
		if text == "" {
			return nil
		}
		return []ChatMessage{{Role: role, Content: text}}
	}

	messages := make([]ChatMessage, 0, len(blocks))
	textParts := make([]string, 0)
	flushText := func() {
		text := strings.Join(textParts, "\n")
		textParts = textParts[:0]
		if text != "" {
			messages = append(messages, ChatMessage{Role: role, Content: text})
		}
	}
	for _, blockValue := range blocks {
		block, ok := blockValue.(map[string]interface{})
		if !ok {
			continue
		}
		switch block["type"] {
		case "text":
			if text, _ := block["text"].(string); text != "" {
				textParts = append(textParts, text)
			}
		case "tool_use":
			flushText()
			messages = append(messages, ChatMessage{Role: "assistant", ToolCalls: []map[string]interface{}{anthropicToolUseMapToOpenAI(block)}})
		case "tool_result":
			flushText()
			toolCallID, _ := block["tool_use_id"].(string)
			messages = append(messages, ChatMessage{Role: "tool", ToolCallID: toolCallID, Content: contentToText(block["content"])})
		}
	}
	flushText()
	return messages
}

func chatMessageToAnthropicMessage(message ChatMessage) (anthropicMessage, bool) {
	role := normalizeRole(message.Role)
	if message.Role == "tool" {
		if message.ToolCallID == "" {
			return anthropicMessage{}, false
		}
		return anthropicMessage{Role: "user", Content: []map[string]interface{}{{
			"type":        "tool_result",
			"tool_use_id": message.ToolCallID,
			"content":     message.Content,
		}}}, true
	}
	if role == "system" {
		role = "user"
	}
	if len(message.ToolCalls) > 0 {
		content := make([]anthropicContent, 0, len(message.ToolCalls)+1)
		if message.Content != "" {
			content = append(content, anthropicContent{Type: "text", Text: message.Content})
		}
		for _, toolCall := range message.ToolCalls {
			content = append(content, openAIToolCallToAnthropic(toolCall))
		}
		return anthropicMessage{Role: "assistant", Content: content}, true
	}
	if message.Content == "" {
		return anthropicMessage{}, false
	}
	return anthropicMessage{Role: role, Content: message.Content}, true
}

func openAIToolsToAnthropic(tools []map[string]interface{}) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if function, ok := tool["function"].(map[string]interface{}); ok {
			item := map[string]interface{}{}
			if name, _ := function["name"].(string); name != "" {
				item["name"] = name
			}
			if description, _ := function["description"].(string); description != "" {
				item["description"] = description
			}
			if parameters, ok := function["parameters"]; ok {
				item["input_schema"] = parameters
			} else {
				item["input_schema"] = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			if item["name"] != nil {
				converted = append(converted, item)
			}
			continue
		}
		if name, _ := tool["name"].(string); name != "" {
			converted = append(converted, tool)
		}
	}
	return converted
}

func anthropicToolsToOpenAI(tools []map[string]interface{}) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		function := map[string]interface{}{}
		if name, _ := tool["name"].(string); name != "" {
			function["name"] = name
		}
		if description, _ := tool["description"].(string); description != "" {
			function["description"] = description
		}
		if schema, ok := tool["input_schema"]; ok {
			function["parameters"] = schema
		} else if parameters, ok := tool["parameters"]; ok {
			function["parameters"] = parameters
		} else {
			function["parameters"] = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		if function["name"] != nil {
			converted = append(converted, map[string]interface{}{"type": "function", "function": function})
		}
	}
	return converted
}

func openAIToolChoiceToAnthropic(choice interface{}) interface{} {
	switch value := choice.(type) {
	case nil:
		return nil
	case string:
		if value == "none" {
			return map[string]interface{}{"type": "none"}
		}
		if value == "required" {
			return map[string]interface{}{"type": "any"}
		}
		return map[string]interface{}{"type": "auto"}
	case map[string]interface{}:
		if function, ok := value["function"].(map[string]interface{}); ok {
			if name, _ := function["name"].(string); name != "" {
				return map[string]interface{}{"type": "tool", "name": name}
			}
		}
	}
	return nil
}

func anthropicToolChoiceToOpenAI(choice interface{}) interface{} {
	value, ok := choice.(map[string]interface{})
	if !ok {
		return choice
	}
	switch value["type"] {
	case "none":
		return "none"
	case "any":
		return "required"
	case "tool":
		if name, _ := value["name"].(string); name != "" {
			return map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": name}}
		}
	}
	return "auto"
}

func anthropicToolUseToOpenAI(item anthropicContent) map[string]interface{} {
	return map[string]interface{}{
		"id":   item.ID,
		"type": "function",
		"function": map[string]interface{}{
			"name":      item.Name,
			"arguments": jsonString(item.Input),
		},
	}
}

func anthropicToolUseMapToOpenAI(item map[string]interface{}) map[string]interface{} {
	id, _ := item["id"].(string)
	name, _ := item["name"].(string)
	return map[string]interface{}{
		"id":   id,
		"type": "function",
		"function": map[string]interface{}{
			"name":      name,
			"arguments": jsonString(item["input"]),
		},
	}
}

func openAIToolCallToAnthropic(toolCall map[string]interface{}) anthropicContent {
	function, _ := toolCall["function"].(map[string]interface{})
	id, _ := toolCall["id"].(string)
	name, _ := function["name"].(string)
	input := parseJSONValue(function["arguments"])
	return anthropicContent{Type: "tool_use", ID: id, Name: name, Input: input}
}

func jsonString(value interface{}) string {
	if value == nil {
		return "{}"
	}
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseJSONValue(value interface{}) interface{} {
	text, ok := value.(string)
	if !ok {
		return value
	}
	if strings.TrimSpace(text) == "" {
		return map[string]interface{}{}
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return map[string]interface{}{}
	}
	return parsed
}

func finishReasonToAnthropic(reason string) string {
	switch normalizeFinishReason(reason) {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}
