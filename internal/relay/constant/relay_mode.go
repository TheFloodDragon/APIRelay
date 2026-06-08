package constant

// RelayMode 表示当前 /v1 入口对应的能力类型。
type RelayMode string

const (
	RelayModeMessages         RelayMode = "messages"
	RelayModeChatCompletions  RelayMode = "chat_completions"
	RelayModeResponses        RelayMode = "responses"
	RelayModeResponsesCompact RelayMode = "responses_compact"
	RelayModeCompletions      RelayMode = "completions"
	RelayModeEmbeddings       RelayMode = "embeddings"
	RelayModeGeminiNative     RelayMode = "gemini_native"
	RelayModeModels           RelayMode = "models"
	RelayModeCountTokens      RelayMode = "count_tokens"
)

func (m RelayMode) String() string {
	return string(m)
}

func (m RelayMode) IsChatLike() bool {
	switch m {
	case RelayModeMessages, RelayModeChatCompletions, RelayModeGeminiNative:
		return true
	default:
		return false
	}
}
