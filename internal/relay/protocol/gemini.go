package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

type geminiGenerateContentRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiContent           `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings    []geminiSafetySetting    `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiGenerateContentResponse struct {
	Candidates     []geminiCandidate    `json:"candidates"`
	PromptFeedback *geminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *geminiUsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string                `json:"modelVersion,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
	Index        int           `json:"index"`
}

type geminiPromptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount      int `json:"totalTokenCount,omitempty"`
}

// GeminiGenerateContentRequestToProtocol 将 Gemini generateContent 请求转为通用文本聊天请求。
func GeminiGenerateContentRequestToProtocol(body []byte, model string, stream bool) (*ChatRequest, error) {
	var req geminiGenerateContentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("解析 Gemini 请求失败: %w", err)
	}

	chatReq := &ChatRequest{Model: model, Stream: stream}
	if req.GenerationConfig != nil {
		chatReq.Temperature = req.GenerationConfig.Temperature
		chatReq.TopP = req.GenerationConfig.TopP
		chatReq.MaxTokens = req.GenerationConfig.MaxOutputTokens
		chatReq.Stop = req.GenerationConfig.StopSequences
	}
	if req.SystemInstruction != nil {
		chatReq.System = geminiContentText(*req.SystemInstruction)
	}
	for _, content := range req.Contents {
		text := geminiContentText(content)
		if text == "" {
			continue
		}
		role := "user"
		if strings.ToLower(content.Role) == "model" || strings.ToLower(content.Role) == "assistant" {
			role = "assistant"
		}
		chatReq.Messages = append(chatReq.Messages, ChatMessage{Role: role, Content: text})
	}
	if len(chatReq.Messages) == 0 && chatReq.System == "" {
		return nil, fmt.Errorf("缺少可转发的 contents")
	}
	return chatReq, nil
}

// ProtocolToGeminiGenerateContentRequest 将通用文本聊天请求转为 Gemini generateContent 请求。
func ProtocolToGeminiGenerateContentRequest(req *ChatRequest) ([]byte, error) {
	geminiReq := geminiGenerateContentRequest{
		GenerationConfig: &geminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
		SafetySettings: defaultGeminiSafetySettings(),
	}
	if req.System != "" {
		geminiReq.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: req.System}}}
	}
	for _, message := range req.Messages {
		role := "user"
		if normalizeRole(message.Role) == "assistant" {
			role = "model"
		}
		geminiReq.Contents = append(geminiReq.Contents, geminiContent{Role: role, Parts: []geminiPart{{Text: message.Content}}})
	}
	if len(geminiReq.Contents) == 0 && req.System != "" {
		geminiReq.Contents = append(geminiReq.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: req.System}}})
		geminiReq.SystemInstruction = nil
	}
	return json.Marshal(geminiReq)
}

// GeminiGenerateContentResponseToProtocol 将 Gemini generateContent 响应转为通用响应。
func GeminiGenerateContentResponseToProtocol(body []byte) (*ChatResponse, error) {
	var resp geminiGenerateContentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Gemini 响应失败: %w", err)
	}
	if len(resp.Candidates) == 0 {
		if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
			return nil, fmt.Errorf("Gemini 响应被阻止: %s", resp.PromptFeedback.BlockReason)
		}
		return nil, fmt.Errorf("Gemini 响应中没有候选结果")
	}

	candidate := resp.Candidates[0]
	chatResp := &ChatResponse{
		ID:           generatedID("chatcmpl"),
		Model:        resp.ModelVersion,
		Role:         "assistant",
		Content:      geminiContentText(candidate.Content),
		FinishReason: normalizeFinishReason(candidate.FinishReason),
		Created:      nowUnix(),
	}
	if resp.UsageMetadata != nil {
		chatResp.Usage = &Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}
	return chatResp, nil
}

// ProtocolToGeminiGenerateContentResponse 将通用响应转为 Gemini generateContent 响应。
func ProtocolToGeminiGenerateContentResponse(resp *ChatResponse) ([]byte, error) {
	geminiResp := geminiGenerateContentResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: resp.Content}},
				},
				FinishReason: finishReasonToGemini(resp.FinishReason),
				Index:        0,
			},
		},
	}
	if resp.Model != "" {
		geminiResp.ModelVersion = resp.Model
	}
	if resp.Usage != nil {
		geminiResp.UsageMetadata = &geminiUsageMetadata{
			PromptTokenCount:     resp.Usage.PromptTokens,
			CandidatesTokenCount: resp.Usage.CompletionTokens,
			TotalTokenCount:      resp.Usage.TotalTokens,
		}
	}
	return json.Marshal(geminiResp)
}

// GeminiStreamEventsFromData 将单条 Gemini SSE data 负载转为通用流事件。
func GeminiStreamEventsFromData(data string) ([]StreamEvent, error) {
	if data == "[DONE]" {
		return []StreamEvent{{Done: true}}, nil
	}
	data = strings.TrimSpace(strings.TrimSuffix(data, ","))
	if data == "" || data == "[" || data == "]" {
		return nil, nil
	}

	var resp geminiGenerateContentResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, err
	}
	if len(resp.Candidates) == 0 {
		return nil, nil
	}

	events := make([]StreamEvent, 0, len(resp.Candidates))
	for _, candidate := range resp.Candidates {
		event := StreamEvent{
			Model:        resp.ModelVersion,
			Content:      geminiContentText(candidate.Content),
			FinishReason: normalizeFinishReason(candidate.FinishReason),
			Index:        candidate.Index,
		}
		if event.Content != "" || candidate.FinishReason != "" {
			events = append(events, event)
		}
	}
	return events, nil
}

// ProtocolStreamEventToGeminiData 将通用流事件编码为 Gemini SSE data 块。
func ProtocolStreamEventToGeminiData(event StreamEvent) ([]byte, error) {
	if event.Done {
		return nil, nil
	}
	if event.Content == "" && event.FinishReason == "" {
		return nil, nil
	}
	candidate := geminiCandidate{Index: event.Index}
	if event.Content != "" {
		candidate.Content = geminiContent{Role: "model", Parts: []geminiPart{{Text: event.Content}}}
	}
	if event.FinishReason != "" {
		candidate.FinishReason = finishReasonToGemini(event.FinishReason)
	}
	resp := geminiGenerateContentResponse{Candidates: []geminiCandidate{candidate}}
	if event.Model != "" {
		resp.ModelVersion = event.Model
	}
	chunkBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return append(append([]byte("data: "), chunkBytes...), []byte("\n\n")...), nil
}

func geminiContentText(content geminiContent) string {
	parts := make([]string, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}

func defaultGeminiSafetySettings() []geminiSafetySetting {
	return []geminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	}
}

func finishReasonToGemini(reason string) string {
	switch normalizeFinishReason(reason) {
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	default:
		return "STOP"
	}
}
