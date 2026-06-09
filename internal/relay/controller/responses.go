package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (rc *RelayController) Responses(c *gin.Context) {
	rc.handleResponsesBridge(c)
}

func (rc *RelayController) CodexResponses(c *gin.Context) {
	rc.handleResponsesBridge(c)
}

func (rc *RelayController) ResponsesCompact(c *gin.Context) {
	writeRelayError(c, http.StatusBadRequest, "responses compact is not supported yet; use /responses for compatible Codex requests", "unsupported_relay_mode", "compact mode is not implemented by APIRelay yet")
}
