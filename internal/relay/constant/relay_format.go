package constant

// RelayFormat 表示客户端请求进入 APIRelay 时使用的协议格式。
type RelayFormat string

const (
	RelayFormatOpenAI          RelayFormat = "openai"
	RelayFormatOpenAIResponses RelayFormat = "openai_responses"
)

func (f RelayFormat) String() string {
	return string(f)
}
