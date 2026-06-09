package controller

import (
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

type responsesUpstreamMode string

const (
	responsesModeAuto       responsesUpstreamMode = "auto"
	responsesModeNative     responsesUpstreamMode = "native"
	responsesModeChatBridge responsesUpstreamMode = "chat_bridge"
)

type responsesAttemptKind string

const (
	responsesAttemptNative     responsesAttemptKind = "native"
	responsesAttemptChatBridge responsesAttemptKind = "chat_bridge"
)

func responsesAttemptOrder(candidate relayCandidate) []responsesAttemptKind {
	switch configuredResponsesMode(candidate.Channel) {
	case responsesModeNative:
		return []responsesAttemptKind{responsesAttemptNative}
	case responsesModeChatBridge:
		return []responsesAttemptKind{responsesAttemptChatBridge}
	default:
		if shouldAutoTryNativeResponses(candidate.Channel) {
			return []responsesAttemptKind{responsesAttemptNative, responsesAttemptChatBridge}
		}
		return []responsesAttemptKind{responsesAttemptChatBridge}
	}
}

func configuredResponsesMode(channel model.Channel) responsesUpstreamMode {
	mode := strings.ToLower(strings.TrimSpace(channelConfigString(channel.Config, "responses_mode")))
	switch mode {
	case "native", "responses", "openai_responses":
		return responsesModeNative
	case "chat", "chat_bridge", "chat-bridge", "bridge":
		return responsesModeChatBridge
	default:
		return responsesModeAuto
	}
}

func shouldAutoTryNativeResponses(channel model.Channel) bool {
	// OpenAI 兼容渠道（NewAPI/OneAPI/第三方中转等）即使声明了
	// supports_responses，也经常只兼容 /chat/completions；直接转发到
	// /responses 会得到 openai_error，而 CC-Switch 的可用路径通常是
	// Responses -> Chat Completions 桥接。因此 auto 模式只对 OpenAI 官方
	// 渠道尝试原生 Responses，兼容渠道默认走 chat_bridge。
	if !strings.EqualFold(strings.TrimSpace(channel.Type), "openai") {
		return false
	}
	if constant.APITypeFromChannelType(channel.Type) != constant.APITypeOpenAI {
		return false
	}
	return channelConfigBool(channel.Config, "supports_responses")
}

func channelConfigString(config model.JSONMap, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func channelConfigBool(config model.JSONMap, key string) bool {
	if config == nil {
		return false
	}
	value, ok := config[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "on":
			return true
		default:
			return false
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}
