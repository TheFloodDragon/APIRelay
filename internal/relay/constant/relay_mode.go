package constant

// RelayMode 表示当前 /v1 入口对应的能力类型。
type RelayMode string

const (
	RelayModeChatCompletions RelayMode = "chat_completions"
	RelayModeResponses       RelayMode = "responses"
	RelayModeCompletions     RelayMode = "completions"
	RelayModeEmbeddings      RelayMode = "embeddings"
)

func (m RelayMode) String() string {
	return string(m)
}
