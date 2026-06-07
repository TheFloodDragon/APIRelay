package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func writeRelayError(c *gin.Context, statusCode int, message, errorType, details string) {
	errorBody := gin.H{
		"message": message,
		"type":    errorType,
	}
	if details != "" {
		errorBody["details"] = details
	}
	c.JSON(statusCode, gin.H{"error": errorBody})
}

func writeFinalRelayError(c *gin.Context, lastErr error, lastErrMsg string, attemptedUpstream bool) {
	details := lastErrMsg
	if details == "" && lastErr != nil {
		details = lastErr.Error()
	}
	if details == "" {
		details = "上游渠道返回非成功状态码"
	}

	if !attemptedUpstream && isUnsupportedRelayModeMessage(details) {
		writeRelayError(c, http.StatusBadRequest, details, "unsupported_relay_mode", "")
		return
	}

	writeRelayError(c, http.StatusServiceUnavailable, "所有渠道请求失败", "api_error", details)
}

func isUnsupportedRelayModeError(err error) bool {
	if err == nil {
		return false
	}
	return isUnsupportedRelayModeMessage(err.Error())
}

func isUnsupportedRelayModeMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "not supported") || strings.Contains(message, "unsupported")
}
