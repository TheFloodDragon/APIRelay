package controller

import "github.com/gin-gonic/gin"

func (rc *RelayController) Responses(c *gin.Context) {
	rc.handleResponsesBridge(c)
}
