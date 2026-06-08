package controller

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

func (rc *RelayController) Responses(c *gin.Context) {
	rc.handleResponsesBridgeWithApp(c, constant.RelayAppOpenAI)
}

func (rc *RelayController) CodexResponses(c *gin.Context) {
	rc.handleResponsesBridgeWithApp(c, constant.RelayAppCodex)
}

func (rc *RelayController) ResponsesCompact(c *gin.Context) {
	writeRelayError(c, http.StatusNotImplemented, "responses compact is not supported yet", "unsupported_relay_mode", "")
}
