package constant

// EndpointType 表示中转站【对外暴露】的协议端点类型。
// 它与上游渠道的实际协议（APIType）解耦。
type EndpointType string

const (
	// EndpointOpenAI 对应 POST /v1/chat/completions
	EndpointOpenAI EndpointType = "openai"
	// EndpointAnthropic 对应 POST /v1/messages
	EndpointAnthropic EndpointType = "anthropic"
	// EndpointResponses 对应 POST /v1/responses
	EndpointResponses EndpointType = "responses"
)
