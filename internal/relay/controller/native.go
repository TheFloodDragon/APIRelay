package controller

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

// AnthropicMessages 提供 Anthropic Messages 兼容入口：/v1/messages。
func (rc *RelayController) AnthropicMessages(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppClaude, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}

// ClaudeMessages 提供带 /claude 命名空间的 Anthropic Messages 兼容入口。
func (rc *RelayController) ClaudeMessages(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppClaude, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}

// GeminiGenerateContent 提供 Gemini generateContent / streamGenerateContent 兼容入口。
func (rc *RelayController) GeminiGenerateContent(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppGemini, constant.RelayModeGeminiNative, constant.RelayFormatGemini)
}

// GeminiNative 提供带 /gemini 命名空间的 Gemini Native 兼容入口。
func (rc *RelayController) GeminiNative(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppGemini, constant.RelayModeGeminiNative, constant.RelayFormatGemini)
}
