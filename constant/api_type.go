package constant

// APIType 表示上游渠道【实际使用】的协议类型。
// 通过 ChannelType2APIType 由渠道类型映射得到，决定使用哪个 Adaptor。
type APIType int

const (
	APITypeOpenAI    APIType = iota // OpenAI Chat Completions 兼容
	APITypeAnthropic                // Anthropic Messages
	APITypeResponses                // OpenAI Responses
)

// 渠道类型常量。新增上游平台时在此扩展，并在 ChannelType2APIType 中映射。
const (
	ChannelTypeOpenAI    = 1 // 通用 OpenAI 兼容（含 OpenAI、DeepSeek、Moonshot 等）
	ChannelTypeAnthropic = 2 // Claude /v1/messages
	ChannelTypeResponses = 3 // OpenAI Responses /v1/responses
)

// ChannelType2APIType 将渠道类型映射为上游协议类型。
// 第二个返回值表示映射是否命中（未命中默认按 OpenAI 处理）。
func ChannelType2APIType(channelType int) (APIType, bool) {
	switch channelType {
	case ChannelTypeOpenAI:
		return APITypeOpenAI, true
	case ChannelTypeAnthropic:
		return APITypeAnthropic, true
	case ChannelTypeResponses:
		return APITypeResponses, true
	default:
		return APITypeOpenAI, false
	}
}

// ChannelTypeName 返回渠道类型的可读名称。
func ChannelTypeName(channelType int) string {
	switch channelType {
	case ChannelTypeOpenAI:
		return "OpenAI"
	case ChannelTypeAnthropic:
		return "Anthropic"
	case ChannelTypeResponses:
		return "OpenAI-Responses"
	default:
		return "Unknown"
	}
}
