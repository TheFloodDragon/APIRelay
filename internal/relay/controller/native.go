package controller

import (
	"fmt"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/gin-gonic/gin"
)

// AnthropicMessages 提供 Anthropic Messages 兼容入口：/v1/messages。
func (rc *RelayController) AnthropicMessages(c *gin.Context) {
	rc.handleRelay(c, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}

// ClaudeMessages 提供带 /claude 命名空间的 Anthropic Messages 兼容入口。
func (rc *RelayController) ClaudeMessages(c *gin.Context) {
	rc.handleRelay(c, constant.RelayModeMessages, constant.RelayFormatAnthropic)
}

// GeminiGenerateContent 提供 Gemini generateContent / streamGenerateContent 兼容入口。
func (rc *RelayController) GeminiGenerateContent(c *gin.Context) {
	rc.handleGeminiNative(c)
}

// GeminiNative 提供带 /gemini 命名空间的 Gemini Native 兼容入口。
func (rc *RelayController) GeminiNative(c *gin.Context) {
	rc.handleGeminiNative(c)
}

func (rc *RelayController) handleGeminiNative(c *gin.Context) {
	route, err := parseGeminiNativePath(c.Request.URL.Path, c.Request.URL.RawQuery)
	if err != nil {
		writeGeminiError(c, http.StatusBadRequest, err.Error(), "INVALID_ARGUMENT")
		return
	}

	switch route.Kind {
	case geminiNativeRouteModels:
		if !geminiMethodAllowed(c, http.MethodGet, route.Kind) {
			return
		}
		rc.GetGeminiModels(c)
	case geminiNativeRouteModel:
		if !geminiMethodAllowed(c, http.MethodGet, route.Kind) {
			return
		}
		rc.writeGeminiModel(c, route.Model)
	case geminiNativeRouteGenerate:
		if !geminiMethodAllowed(c, http.MethodPost, route.Kind) {
			return
		}
		rc.handleRelay(c, constant.RelayModeGeminiNative, constant.RelayFormatGemini)
	case geminiNativeRouteCountTokens:
		if !geminiMethodAllowed(c, http.MethodPost, route.Kind) {
			return
		}
		rc.handleGeminiCountTokens(c)
	default:
		writeGeminiError(c, http.StatusBadRequest, "不支持的 Gemini 路径", "INVALID_ARGUMENT")
	}
}

func (rc *RelayController) handleGeminiCountTokens(c *gin.Context) {
	reqCtx, ok := rc.newRequestContext(c, constant.RelayModeCountTokens, constant.RelayFormatGemini)
	if !ok {
		return
	}
	rc.relayJSON(reqCtx)
}

func geminiMethodAllowed(c *gin.Context, expectedMethod string, kind geminiNativeRouteKind) bool {
	if c.Request.Method == expectedMethod {
		return true
	}
	writeGeminiError(
		c,
		http.StatusMethodNotAllowed,
		fmt.Sprintf("Gemini %s 需要使用 %s 方法", kind, expectedMethod),
		"METHOD_NOT_ALLOWED",
	)
	return false
}
