package controller

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) ChatCompletions(c *gin.Context) {
	rc.handleRelay(c, constant.RelayModeChatCompletions, constant.RelayFormatOpenAI)
}
