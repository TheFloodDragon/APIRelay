package constant

import "strings"

// APIType 表示上游渠道实际使用的协议类型。
type APIType string

const (
	APITypeOpenAI    APIType = "openai"
	APITypeAnthropic APIType = "anthropic"
	APITypeGemini    APIType = "gemini"
)

func (t APIType) String() string {
	return string(t)
}

// APITypeFromChannelType 将可自由输入的渠道类型归一到上游协议类型。
func APITypeFromChannelType(channelType string) APIType {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "anthropic", "claude":
		return APITypeAnthropic
	case "gemini", "google":
		return APITypeGemini
	case "", "openai", "openai_compatible", "newapi", "oneapi", "deepseek", "openrouter", "ollama", "custom", "codex":
		return APITypeOpenAI
	default:
		return APITypeOpenAI
	}
}
