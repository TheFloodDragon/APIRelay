package adapter

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// ModelFetcher 模型获取器接口
type ModelFetcher interface {
	FetchModels() ([]string, error)
}

// GetModelFetcher 获取模型获取器工厂方法
func GetModelFetcher(channelType, apiKey, baseURL string) ModelFetcher {
	switch channelType {
	case "openai", "openai_compatible", "deepseek", "codex":
		return NewOpenAIFetcher(apiKey, baseURL)
	case "anthropic":
		return NewClaudeFetcher(apiKey, baseURL)
	case "gemini":
		return NewGeminiFetcher(apiKey, baseURL)
	default:
		return NewOpenAIFetcher(apiKey, baseURL)
	}
}

// OpenAIFetcher OpenAI模型获取器
type OpenAIFetcher struct {
	APIKey  string
	BaseURL string
	Client  *resty.Client
}

func NewOpenAIFetcher(apiKey, baseURL string) *OpenAIFetcher {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIFetcher{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Client:  resty.New().SetTimeout(10 * time.Second),
	}
}

func (f *OpenAIFetcher) FetchModels() ([]string, error) {
	resp, err := f.Client.R().
		SetHeader("Authorization", "Bearer "+f.APIKey).
		Get(f.BaseURL + "/models")

	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API返回错误: %d - %s", resp.StatusCode(), resp.String())
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// ClaudeFetcher Claude模型获取器
type ClaudeFetcher struct {
	APIKey  string
	BaseURL string
}

func NewClaudeFetcher(apiKey, baseURL string) *ClaudeFetcher {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &ClaudeFetcher{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (f *ClaudeFetcher) FetchModels() ([]string, error) {
	// Claude API 没有 models 端点，返回预定义列表
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"claude-2.1",
		"claude-2.0",
	}, nil
}

// GeminiFetcher Gemini模型获取器
type GeminiFetcher struct {
	APIKey  string
	BaseURL string
}

func NewGeminiFetcher(apiKey, baseURL string) *GeminiFetcher {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiFetcher{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (f *GeminiFetcher) FetchModels() ([]string, error) {
	// Gemini 预定义模型列表
	return []string{
		"gemini-2.0-flash-exp",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-1.0-pro",
	}, nil
}
