package controller

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) ChatCompletions(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppOpenAI, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}

func (rc *RelayController) CodexChatCompletions(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppCodex, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}
