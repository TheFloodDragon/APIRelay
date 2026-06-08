package controller

import (
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) Completions(c *gin.Context) {
	rc.handleRelay(c, constant.RelayAppOpenAI, constant.RelayModeCompletions, constant.RelayFormatOpenAI)
}
