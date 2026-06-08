package constant

// RelayApp 表示当前请求来源客户端/协议命名空间。
type RelayApp string

const (
	RelayAppOpenAI        RelayApp = "openai"
	RelayAppClaude        RelayApp = "claude"
	RelayAppCodex         RelayApp = "codex"
	RelayAppGemini        RelayApp = "gemini"
	RelayAppClaudeDesktop RelayApp = "claude_desktop"
)

func (a RelayApp) String() string {
	return string(a)
}
