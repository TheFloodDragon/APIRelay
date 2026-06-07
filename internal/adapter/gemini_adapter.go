package adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// GeminiChatRequest Gemini 聊天请求格式
type GeminiChatRequest struct {
	Contents         []GeminiContent          `json:"contents"`
	GenerationConfig *GeminiGenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings   []GeminiSafetySetting    `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Role  string        `json:"role"`
	Parts []GeminiPart  `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiChatResponse Gemini 聊天响应格式
type GeminiChatResponse struct {
	Candidates     []GeminiCandidate   `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason,omitempty"`
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

type GeminiPromptFeedback struct {
	BlockReason   string               `json:"blockReason,omitempty"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiAdapter Gemini 协议适配器
type GeminiAdapter struct{}

// ConvertRequest 将 OpenAI 格式请求转换为 Gemini 格式
func (g *GeminiAdapter) ConvertRequest(openaiReq interface{}) (interface{}, error) {
	// 将 openaiReq 转换为 JSON 再解析
	reqBytes, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化 OpenAI 请求失败: %w", err)
	}

	var oaiReq OpenAIChatRequest
	if err := json.Unmarshal(reqBytes, &oaiReq); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}

	// 构建 Gemini 请求
	geminiReq := GeminiChatRequest{
		Contents: make([]GeminiContent, 0),
	}

	// 配置生成参数
	genConfig := &GeminiGenerationConfig{
		Temperature: oaiReq.Temperature,
		TopP:        oaiReq.TopP,
	}

	if oaiReq.MaxTokens != nil {
		genConfig.MaxOutputTokens = oaiReq.MaxTokens
	}

	// 处理 stop 参数
	if oaiReq.Stop != nil {
		switch stop := oaiReq.Stop.(type) {
		case string:
			genConfig.StopSequences = []string{stop}
		case []string:
			genConfig.StopSequences = stop
		case []interface{}:
			for _, s := range stop {
				if str, ok := s.(string); ok {
					genConfig.StopSequences = append(genConfig.StopSequences, str)
				}
			}
		}
	}

	geminiReq.GenerationConfig = genConfig

	// 转换消息格式
	// Gemini 要求消息必须是 user/model 交替出现
	for _, msg := range oaiReq.Messages {
		role := msg.Role

		// 转换角色名称
		if role == "assistant" {
			role = "model"
		} else if role == "system" {
			// Gemini 不支持 system 角色，将其合并到第一条 user 消息
			role = "user"
		}

		geminiReq.Contents = append(geminiReq.Contents, GeminiContent{
			Role: role,
			Parts: []GeminiPart{
				{Text: msg.Content},
			},
		})
	}

	// 设置安全设置为最宽松（避免内容被屏蔽）
	geminiReq.SafetySettings = []GeminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	}

	return geminiReq, nil
}

// ConvertResponse 将 Gemini 格式响应转换为 OpenAI 格式
func (g *GeminiAdapter) ConvertResponse(targetResp io.Reader) ([]byte, error) {
	respBytes, err := io.ReadAll(targetResp)
	if err != nil {
		return nil, err
	}

	var geminiResp GeminiChatResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("解析 Gemini 响应失败: %w", err)
	}

	// 检查是否有候选结果
	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("Gemini 响应中没有候选结果")
	}

	// 提取第一个候选结果的文本
	candidate := geminiResp.Candidates[0]
	var content string
	for _, part := range candidate.Content.Parts {
		content += part.Text
	}

	// 转换为 OpenAI 格式
	openaiResp := OpenAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gemini-pro",
		Choices: []OpenAIChatChoice{
			{
				Index: 0,
				Message: &OpenAIChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: stringPtr(g.convertFinishReason(candidate.FinishReason)),
			},
		},
	}

	// 转换 token 使用量
	if geminiResp.UsageMetadata != nil {
		openaiResp.Usage = &OpenAIUsage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return json.Marshal(openaiResp)
}

// ConvertStreamChunk 将 Gemini 流式响应转换为 OpenAI SSE 格式
func (g *GeminiAdapter) ConvertStreamChunk(targetChunk []byte) ([]byte, error) {
	// Gemini 使用 JSON Lines 格式，每行是一个完整的 JSON 对象
	scanner := bufio.NewScanner(bytes.NewReader(targetChunk))
	var result bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// 跳过空行
		if line == "" {
			continue
		}

		// 解析 Gemini 响应
		var geminiResp GeminiChatResponse
		if err := json.Unmarshal([]byte(line), &geminiResp); err != nil {
			// 如果解析失败，跳过这个块
			continue
		}

		// 检查是否有候选结果
		if len(geminiResp.Candidates) == 0 {
			continue
		}

		candidate := geminiResp.Candidates[0]

		// 提取文本内容
		var content string
		for _, part := range candidate.Content.Parts {
			content += part.Text
		}

		// 转换为 OpenAI 流式格式
		openaiChunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "gemini-pro",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": content,
					},
					"finish_reason": nil,
				},
			},
		}

		// 如果有结束原因，设置 finish_reason
		if candidate.FinishReason != "" {
			openaiChunk["choices"].([]map[string]interface{})[0]["finish_reason"] = g.convertFinishReason(candidate.FinishReason)
			openaiChunk["choices"].([]map[string]interface{})[0]["delta"] = map[string]interface{}{}
		}

		chunkBytes, err := json.Marshal(openaiChunk)
		if err != nil {
			continue
		}

		result.WriteString("data: ")
		result.Write(chunkBytes)
		result.WriteString("\n\n")
	}

	return result.Bytes(), nil
}

// NeedsConversion 判断该渠道类型是否需要协议转换
func (g *GeminiAdapter) NeedsConversion() bool {
	return true
}

// convertFinishReason 转换结束原因
func (g *GeminiAdapter) convertFinishReason(geminiReason string) string {
	switch geminiReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	default:
		return "stop"
	}
}
