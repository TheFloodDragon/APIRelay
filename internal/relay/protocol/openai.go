package protocol

import (
	"encoding/json"
	"fmt"
)

type openAIChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Temperature         *float64            `json:"temperature,omitempty"`
	TopP                *float64            `json:"top_p,omitempty"`
	Stream              bool                `json:"stream,omitempty"`
	Stop                interface{}         `json:"stop,omitempty"`
	MaxTokens           *int                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                `json:"max_completion_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type openAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []openAIChatChoice `json:"choices"`
	Usage   *openAIUsage       `json:"usage,omitempty"`
}

type openAIChatChoice struct {
	Index        int                `json:"index"`
	Message      *openAIChatMessage `json:"message,omitempty"`
	Delta        map[string]string  `json:"delta,omitempty"`
	FinishReason *string            `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIChatRequestToProtocol 将 OpenAI Chat Completions 请求转为通用文本聊天请求。
func OpenAIChatRequestToProtocol(body []byte) (*ChatRequest, error) {
	var req openAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}

	chatReq := &ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        stringList(req.Stop),
		MaxTokens:   req.MaxTokens,
	}
	if chatReq.MaxTokens == nil {
		chatReq.MaxTokens = req.MaxCompletionTokens
	}

	systemParts := make([]string, 0)
	for _, message := range req.Messages {
		role := normalizeRole(message.Role)
		text := contentToText(message.Content)
		if text == "" {
			continue
		}
		if role == "system" {
			systemParts = append(systemParts, text)
			continue
		}
		chatReq.Messages = append(chatReq.Messages, ChatMessage{Role: role, Content: text})
	}
	chatReq.System = textParts(systemParts, "\n\n")

	if len(chatReq.Messages) == 0 && chatReq.System == "" {
		return nil, fmt.Errorf("缺少可转发的 messages")
	}
	return chatReq, nil
}

// ProtocolToOpenAIChatRequest 将通用文本聊天请求转为 OpenAI Chat Completions 请求。
func ProtocolToOpenAIChatRequest(req *ChatRequest) ([]byte, error) {
	openAIReq := openAIChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		MaxTokens:   req.MaxTokens,
	}
	if len(req.Stop) > 0 {
		openAIReq.Stop = req.Stop
	}
	if req.System != "" {
		openAIReq.Messages = append(openAIReq.Messages, openAIChatMessage{Role: "system", Content: req.System})
	}
	for _, message := range req.Messages {
		role := normalizeRole(message.Role)
		if role == "system" {
			role = "user"
		}
		openAIReq.Messages = append(openAIReq.Messages, openAIChatMessage{Role: role, Content: message.Content})
	}
	return json.Marshal(openAIReq)
}

// OpenAIChatResponseToProtocol 将 OpenAI Chat Completions 响应转为通用响应。
func OpenAIChatResponseToProtocol(body []byte) (*ChatResponse, error) {
	var resp openAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}

	chatResp := &ChatResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Role:    "assistant",
		Created: resp.Created,
	}
	if chatResp.ID == "" {
		chatResp.ID = generatedID("chatcmpl")
	}
	if chatResp.Created == 0 {
		chatResp.Created = nowUnix()
	}
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message != nil {
			chatResp.Role = firstNonEmpty(choice.Message.Role, "assistant")
			chatResp.Content = contentToText(choice.Message.Content)
		}
		if choice.FinishReason != nil {
			chatResp.FinishReason = normalizeFinishReason(*choice.FinishReason)
		}
	}
	if chatResp.FinishReason == "" {
		chatResp.FinishReason = "stop"
	}
	if resp.Usage != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return chatResp, nil
}

// ProtocolToOpenAIChatResponse 将通用响应转为 OpenAI Chat Completions 响应。
func ProtocolToOpenAIChatResponse(resp *ChatResponse) ([]byte, error) {
	finishReason := normalizeFinishReason(resp.FinishReason)
	openAIResp := openAIChatResponse{
		ID:      firstNonEmpty(resp.ID, generatedID("chatcmpl")),
		Object:  "chat.completion",
		Created: resp.Created,
		Model:   resp.Model,
		Choices: []openAIChatChoice{
			{
				Index: 0,
				Message: &openAIChatMessage{
					Role:    firstNonEmpty(resp.Role, "assistant"),
					Content: resp.Content,
				},
				FinishReason: &finishReason,
			},
		},
	}
	if openAIResp.Created == 0 {
		openAIResp.Created = nowUnix()
	}
	if resp.Usage != nil {
		openAIResp.Usage = &openAIUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return json.Marshal(openAIResp)
}

// OpenAIStreamEventsFromData 将单条 OpenAI SSE data 负载转为通用流事件。
func OpenAIStreamEventsFromData(data string) ([]StreamEvent, error) {
	if data == "[DONE]" {
		return []StreamEvent{{Done: true}}, nil
	}

	var chunk struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int                    `json:"index"`
			Delta        map[string]interface{} `json:"delta"`
			FinishReason *string                `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, err
	}

	events := make([]StreamEvent, 0, len(chunk.Choices))
	for _, choice := range chunk.Choices {
		event := StreamEvent{ID: chunk.ID, Model: chunk.Model, Index: choice.Index}
		if role, _ := choice.Delta["role"].(string); role != "" {
			event.Role = role
			event.Start = true
		}
		if content, _ := choice.Delta["content"].(string); content != "" {
			event.Content = content
		}
		if choice.FinishReason != nil {
			event.FinishReason = normalizeFinishReason(*choice.FinishReason)
		}
		if event.Start || event.Content != "" || event.FinishReason != "" {
			events = append(events, event)
		}
	}
	return events, nil
}

// ProtocolStreamEventToOpenAIData 将通用流事件编码为 OpenAI SSE data 块。
func ProtocolStreamEventToOpenAIData(event StreamEvent) ([]byte, error) {
	if event.Done {
		return []byte("data: [DONE]\n\n"), nil
	}

	delta := map[string]string{}
	if event.Role != "" {
		delta["role"] = normalizeRole(event.Role)
	}
	if event.Content != "" {
		delta["content"] = event.Content
	}
	var finishReason *string
	if event.FinishReason != "" {
		value := normalizeFinishReason(event.FinishReason)
		finishReason = &value
	}

	chunk := openAIChatResponse{
		ID:      firstNonEmpty(event.ID, generatedID("chatcmpl")),
		Object:  "chat.completion.chunk",
		Created: nowUnix(),
		Model:   event.Model,
		Choices: []openAIChatChoice{
			{Index: event.Index, Delta: delta, FinishReason: finishReason},
		},
	}
	chunkBytes, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}
	return append(append([]byte("data: "), chunkBytes...), []byte("\n\n")...), nil
}
