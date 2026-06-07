package controller

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

// AnthropicMessages 提供 Anthropic Messages 兼容入口：/v1/messages。
func (rc *RelayController) AnthropicMessages(c *gin.Context) {
	rc.handleRelay(c, constant.RelayModeChatCompletions, constant.RelayFormatAnthropic)
}

// GeminiGenerateContent 提供 Gemini generateContent / streamGenerateContent 兼容入口。
func (rc *RelayController) GeminiGenerateContent(c *gin.Context) {
	rc.handleRelay(c, constant.RelayModeChatCompletions, constant.RelayFormatGemini)
}
