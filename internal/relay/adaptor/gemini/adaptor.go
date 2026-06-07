package gemini

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type Adaptor struct{}

func NewAdaptor() *Adaptor {
	return &Adaptor{}
}

func (a *Adaptor) APIType() constant.APIType {
	return constant.APITypeGemini
}

func (a *Adaptor) GetRequestURL(baseURL string, mode constant.RelayMode) string {
	return a.GetRequestURLWithModel(baseURL, mode, "gemini-pro", false)
}

func (a *Adaptor) GetRequestURLWithModel(baseURL string, mode constant.RelayMode, model string, stream bool) string {
	baseURL = normalizeBaseURL(baseURL)
	if model == "" {
		model = "gemini-pro"
	}

	methodSuffix := ":generateContent"
	if stream {
		methodSuffix = ":streamGenerateContent?alt=sse"
	}

	if strings.Contains(baseURL, "{model}") {
		return strings.ReplaceAll(baseURL, "{model}", model) + methodSuffixIfMissing(baseURL, methodSuffix)
	}
	if strings.Contains(baseURL, ":generateContent") || strings.Contains(baseURL, ":streamGenerateContent") {
		return baseURL
	}
	if strings.Contains(baseURL, "/models/") {
		return baseURL + methodSuffix
	}
	return baseURL + "/models/" + model + methodSuffix
}

func (a *Adaptor) SetupHeaders(headers http.Header, apiKey string, mode constant.RelayMode) {
	if apiKey != "" {
		headers.Set("x-goog-api-key", apiKey)
	}
	headers.Set("Content-Type", "application/json")
}

func (a *Adaptor) ConvertRequest(req []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode == constant.RelayModeResponses {
		return nil, fmt.Errorf("responses is not supported for anthropic/gemini channels yet")
	}
	if mode != constant.RelayModeChatCompletions {
		return nil, fmt.Errorf("%s is not supported for gemini channels yet", mode)
	}

	var openaiReq openAIChatRequest
	if err := json.Unmarshal(req, &openaiReq); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}

	geminiReq := geminiGenerateContentRequest{
		Contents: make([]geminiContent, 0, len(openaiReq.Messages)),
		GenerationConfig: &geminiGenerationConfig{
			Temperature: openaiReq.Temperature,
			TopP:        openaiReq.TopP,
		},
		SafetySettings: defaultSafetySettings(),
	}
	if openaiReq.MaxTokens != nil {
		geminiReq.GenerationConfig.MaxOutputTokens = openaiReq.MaxTokens
	} else if openaiReq.MaxCompletionTokens != nil {
		geminiReq.GenerationConfig.MaxOutputTokens = openaiReq.MaxCompletionTokens
	}
	geminiReq.GenerationConfig.StopSequences = stringList(openaiReq.Stop)

	var systemTexts []string
	for _, message := range openaiReq.Messages {
		role := strings.ToLower(message.Role)
		text := contentToText(message.Content)
		if text == "" {
			continue
		}
		if role == "system" || role == "developer" {
			systemTexts = append(systemTexts, text)
			continue
		}

		geminiRole := "user"
		if role == "assistant" || role == "model" {
			geminiRole = "model"
		}
		geminiReq.Contents = append(geminiReq.Contents, geminiContent{
			Role:  geminiRole,
			Parts: []geminiPart{{Text: text}},
		})
	}

	if len(systemTexts) > 0 {
		systemText := strings.Join(systemTexts, "\n\n")
		if len(geminiReq.Contents) == 0 {
			geminiReq.Contents = append(geminiReq.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: systemText}}})
		} else if geminiReq.Contents[0].Role == "user" && len(geminiReq.Contents[0].Parts) > 0 {
			geminiReq.Contents[0].Parts[0].Text = systemText + "\n\n" + geminiReq.Contents[0].Parts[0].Text
		} else {
			geminiReq.Contents = append([]geminiContent{{Role: "user", Parts: []geminiPart{{Text: systemText}}}}, geminiReq.Contents...)
		}
	}

	if len(geminiReq.Contents) == 0 {
		return nil, fmt.Errorf("缺少可转发到 Gemini 的 messages")
	}

	return json.Marshal(geminiReq)
}

func (a *Adaptor) ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
		return resp, nil
	}

	var geminiResp geminiGenerateContentResponse
	if err := json.Unmarshal(resp, &geminiResp); err != nil {
		return nil, fmt.Errorf("解析 Gemini 响应失败: %w", err)
	}
	if len(geminiResp.Candidates) == 0 {
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			return nil, fmt.Errorf("Gemini 响应被阻止: %s", geminiResp.PromptFeedback.BlockReason)
		}
		return nil, fmt.Errorf("Gemini 响应中没有候选结果")
	}

	candidate := geminiResp.Candidates[0]
	content := geminiContentText(candidate.Content)
	finishReason := convertFinishReason(candidate.FinishReason)
	openaiResp := openAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gemini",
		Choices: []openAIChatChoice{
			{
				Index: candidate.Index,
				Message: &openAIChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: &finishReason,
			},
		},
	}
	if geminiResp.UsageMetadata != nil {
		openaiResp.Usage = &openAIUsage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}
	return json.Marshal(openaiResp)
}

func (a *Adaptor) ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error) {
	if mode != constant.RelayModeChatCompletions {
		return chunk, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(chunk))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var result bytes.Buffer
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		line = strings.TrimSuffix(line, ",")
		if line == "" || line == "[" || line == "]" || line == "[DONE]" {
			if line == "[DONE]" {
				result.WriteString("data: [DONE]\n\n")
			}
			continue
		}

		var geminiResp geminiGenerateContentResponse
		if err := json.Unmarshal([]byte(line), &geminiResp); err != nil {
			continue
		}
		for _, openaiChunk := range geminiStreamChunks(&geminiResp) {
			if done, _ := openaiChunk["__done"].(bool); done {
				result.WriteString("data: [DONE]\n\n")
				continue
			}
			chunkBytes, err := json.Marshal(openaiChunk)
			if err != nil {
				continue
			}
			result.WriteString("data: ")
			result.Write(chunkBytes)
			result.WriteString("\n\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func (a *Adaptor) ErrorMessage(resp []byte) string {
	return parseErrorMessage(resp)
}

type openAIChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Temperature         *float64            `json:"temperature,omitempty"`
	TopP                *float64            `json:"top_p,omitempty"`
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

type geminiGenerateContentRequest struct {
	Contents         []geminiContent          `json:"contents"`
	GenerationConfig *geminiGenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings   []geminiSafetySetting    `json:"safetySettings,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
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
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

var doneSentinel = map[string]interface{}{"__done": true}

func geminiStreamChunks(resp *geminiGenerateContentResponse) []map[string]interface{} {
	if len(resp.Candidates) == 0 {
		return nil
	}
	candidate := resp.Candidates[0]
	content := geminiContentText(candidate.Content)
	chunks := make([]map[string]interface{}, 0, 2)
	if content != "" {
		chunks = append(chunks, openAIStreamChunk(map[string]string{"content": content}, nil, candidate.Index))
	}
	if candidate.FinishReason != "" {
		finishReason := convertFinishReason(candidate.FinishReason)
		chunks = append(chunks, openAIStreamChunk(map[string]string{}, &finishReason, candidate.Index), doneSentinel)
	}
	return chunks
}

func openAIStreamChunk(delta map[string]string, finishReason *string, index int) map[string]interface{} {
	return map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "gemini",
		"choices": []map[string]interface{}{
			{
				"index":         index,
				"delta":         delta,
				"finish_reason": finishReason,
			},
		},
	}
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func methodSuffixIfMissing(url, suffix string) string {
	if strings.Contains(url, ":generateContent") || strings.Contains(url, ":streamGenerateContent") {
		return ""
	}
	return suffix
}

func defaultSafetySettings() []geminiSafetySetting {
	return []geminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	}
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

func contentToText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok && text != "" {
				parts = append(parts, text)
				continue
			}
			if text, ok := part["input_text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func stringList(value interface{}) []string {
	switch stop := value.(type) {
	case nil:
		return nil
	case string:
		if stop == "" {
			return nil
		}
		return []string{stop}
	case []string:
		return stop
	case []interface{}:
		items := make([]string, 0, len(stop))
		for _, item := range stop {
			if text, ok := item.(string); ok && text != "" {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

func convertFinishReason(reason string) string {
	switch reason {
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "content_filter"
	case "STOP", "":
		fallthrough
	default:
		return "stop"
	}
}

func parseErrorMessage(resp []byte) string {
	if len(resp) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(resp, &payload); err != nil {
		return string(resp)
	}
	if errorValue, ok := payload["error"]; ok {
		switch errObj := errorValue.(type) {
		case map[string]interface{}:
			if message, ok := errObj["message"].(string); ok && message != "" {
				return message
			}
		case string:
			return errObj
		}
	}
	if message, ok := payload["message"].(string); ok && message != "" {
		return message
	}
	return string(resp)
}
