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

// AnthropicChatRequest Anthropic 聊天请求格式
type AnthropicChatRequest struct {
	Model         string                  `json:"model"`
	Messages      []AnthropicMessage      `json:"messages"`
	MaxTokens     int                     `json:"max_tokens"`
	Temperature   *float64                `json:"temperature,omitempty"`
	TopP          *float64                `json:"top_p,omitempty"`
	TopK          *int                    `json:"top_k,omitempty"`
	Stream        bool                    `json:"stream,omitempty"`
	StopSequences []string                `json:"stop_sequences,omitempty"`
	System        string                  `json:"system,omitempty"`
	Metadata      map[string]interface{}  `json:"metadata,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicChatResponse Anthropic 聊天响应格式
type AnthropicChatResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContent      `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage         `json:"usage,omitempty"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicStreamChunk Anthropic 流式响应块
type AnthropicStreamChunk struct {
	Type         string              `json:"type"`
	Index        int                 `json:"index,omitempty"`
	Delta        *AnthropicDelta     `json:"delta,omitempty"`
	ContentBlock *AnthropicContent   `json:"content_block,omitempty"`
	Message      *AnthropicChatResponse `json:"message,omitempty"`
	Usage        *AnthropicUsage     `json:"usage,omitempty"`
}

type AnthropicDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// AnthropicAdapter Claude 协议适配器
type AnthropicAdapter struct{}

// ConvertRequest 将 OpenAI 格式请求转换为 Anthropic 格式
func (a *AnthropicAdapter) ConvertRequest(openaiReq interface{}) (interface{}, error) {
	// 将 openaiReq 转换为 JSON 再解析
	reqBytes, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化 OpenAI 请求失败: %w", err)
	}

	var oaiReq OpenAIChatRequest
	if err := json.Unmarshal(reqBytes, &oaiReq); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 请求失败: %w", err)
	}

	// 构建 Anthropic 请求
	anthropicReq := AnthropicChatRequest{
		Model:       a.convertModelName(oaiReq.Model),
		Temperature: oaiReq.Temperature,
		TopP:        oaiReq.TopP,
		Stream:      oaiReq.Stream,
	}

	// 设置 MaxTokens（Anthropic 必需）
	if oaiReq.MaxTokens != nil {
		anthropicReq.MaxTokens = *oaiReq.MaxTokens
	} else {
		anthropicReq.MaxTokens = 4096 // 默认值
	}

	// 处理 stop 参数
	if oaiReq.Stop != nil {
		switch stop := oaiReq.Stop.(type) {
		case string:
			anthropicReq.StopSequences = []string{stop}
		case []string:
			anthropicReq.StopSequences = stop
		case []interface{}:
			for _, s := range stop {
				if str, ok := s.(string); ok {
					anthropicReq.StopSequences = append(anthropicReq.StopSequences, str)
				}
			}
		}
	}

	// 转换消息格式（提取 system 消息）
	var systemMessage string
	anthropicMessages := make([]AnthropicMessage, 0)

	for _, msg := range oaiReq.Messages {
		if msg.Role == "system" {
			// Anthropic 将 system 消息单独放在 system 字段
			if systemMessage != "" {
				systemMessage += "\n\n"
			}
			systemMessage += msg.Content
		} else {
			anthropicMessages = append(anthropicMessages, AnthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	anthropicReq.System = systemMessage
	anthropicReq.Messages = anthropicMessages

	return anthropicReq, nil
}

// ConvertResponse 将 Anthropic 格式响应转换为 OpenAI 格式
func (a *AnthropicAdapter) ConvertResponse(targetResp io.Reader) ([]byte, error) {
	respBytes, err := io.ReadAll(targetResp)
	if err != nil {
		return nil, err
	}

	var anthropicResp AnthropicChatResponse
	if err := json.Unmarshal(respBytes, &anthropicResp); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 响应失败: %w", err)
	}

	// 提取文本内容
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// 转换为 OpenAI 格式
	openaiResp := OpenAIChatResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []OpenAIChatChoice{
			{
				Index: 0,
				Message: &OpenAIChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: stringPtr(a.convertStopReason(anthropicResp.StopReason)),
			},
		},
	}

	// 转换 token 使用量
	if anthropicResp.Usage != nil {
		openaiResp.Usage = &OpenAIUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		}
	}

	return json.Marshal(openaiResp)
}

// ConvertStreamChunk 将 Anthropic 流式响应转换为 OpenAI SSE 格式
func (a *AnthropicAdapter) ConvertStreamChunk(targetChunk []byte) ([]byte, error) {
	// Anthropic 使用 SSE 格式，每行以 "data: " 开头
	scanner := bufio.NewScanner(bytes.NewReader(targetChunk))
	var result bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 解析 SSE 数据行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// [DONE] 标记
			if data == "[DONE]" {
				result.WriteString("data: [DONE]\n\n")
				continue
			}

			// 解析 Anthropic chunk
			var anthropicChunk AnthropicStreamChunk
			if err := json.Unmarshal([]byte(data), &anthropicChunk); err != nil {
				// 如果解析失败，跳过这个块
				continue
			}

			// 转换为 OpenAI 格式
			openaiChunk := a.convertStreamChunkToOpenAI(&anthropicChunk)
			if openaiChunk != nil {
				chunkBytes, err := json.Marshal(openaiChunk)
				if err != nil {
					continue
				}
				result.WriteString("data: ")
				result.Write(chunkBytes)
				result.WriteString("\n\n")
			}
		} else if strings.HasPrefix(line, "event: ") {
			// 保留事件类型（可选）
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.Bytes(), nil
}

// NeedsConversion 判断该渠道类型是否需要协议转换
func (a *AnthropicAdapter) NeedsConversion() bool {
	return true
}

// convertStreamChunkToOpenAI 转换单个流式块
func (a *AnthropicAdapter) convertStreamChunkToOpenAI(anthropicChunk *AnthropicStreamChunk) interface{} {
	switch anthropicChunk.Type {
	case "message_start":
		// 消息开始
		return map[string]interface{}{
			"id":      "",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   "",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"role": "assistant",
					},
					"finish_reason": nil,
				},
			},
		}

	case "content_block_start":
		// 内容块开始（通常可以忽略）
		return nil

	case "content_block_delta":
		// 内容增量
		if anthropicChunk.Delta != nil && anthropicChunk.Delta.Text != "" {
			return map[string]interface{}{
				"id":      "",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "",
				"choices": []map[string]interface{}{
					{
						"index": anthropicChunk.Index,
						"delta": map[string]interface{}{
							"content": anthropicChunk.Delta.Text,
						},
						"finish_reason": nil,
					},
				},
			}
		}

	case "content_block_stop":
		// 内容块结束（通常可以忽略）
		return nil

	case "message_delta":
		// 消息增量（包含停止原因）
		if anthropicChunk.Delta != nil && anthropicChunk.Delta.StopReason != "" {
			return map[string]interface{}{
				"id":      "",
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   "",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": a.convertStopReason(anthropicChunk.Delta.StopReason),
					},
				},
			}
		}

	case "message_stop":
		// 消息结束
		return nil
	}

	return nil
}

// convertModelName 转换模型名称
func (a *AnthropicAdapter) convertModelName(openaiModel string) string {
	// 常见模型名称映射
	modelMap := map[string]string{
		"gpt-4":            "claude-3-opus-20240229",
		"gpt-4-turbo":      "claude-3-sonnet-20240229",
		"gpt-3.5-turbo":    "claude-3-haiku-20240307",
		"claude-3-opus":    "claude-3-opus-20240229",
		"claude-3-sonnet":  "claude-3-sonnet-20240229",
		"claude-3-haiku":   "claude-3-haiku-20240307",
	}

	if mapped, ok := modelMap[openaiModel]; ok {
		return mapped
	}

	// 如果已经是 Anthropic 格式，直接返回
	if strings.HasPrefix(openaiModel, "claude-") {
		return openaiModel
	}

	// 默认使用 Sonnet
	return "claude-3-sonnet-20240229"
}

// convertStopReason 转换停止原因
func (a *AnthropicAdapter) convertStopReason(anthropicReason string) string {
	switch anthropicReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

func stringPtr(s string) *string {
	return &s
}
