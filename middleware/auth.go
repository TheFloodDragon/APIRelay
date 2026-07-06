package middleware

import (
	"net/http"
	"time"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

// TokenAuth 校验对外 API Key（用于 relay 接口）。
// 通过后将 *model.Token 存入 c.Set("token", ...)。
func TokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := common.TrimBearer(c.GetHeader("Authorization"))
		if key == "" {
			// 兼容 Anthropic 风格的 x-api-key
			key = c.GetHeader("x-api-key")
		}
		if key == "" {
			abortAuth(c, "missing api key")
			return
		}

		tok, err := model.GetTokenByKey(key)
		if err != nil {
			abortAuth(c, "invalid api key")
			return
		}
		if tok.Status != model.TokenStatusEnabled {
			abortAuth(c, "token disabled")
			return
		}
		if tok.ExpiredAt > 0 && tok.ExpiredAt < time.Now().UnixMilli() {
			abortAuth(c, "token expired")
			return
		}
		if !tok.Unlimited && tok.Quota <= 0 {
			abortAuth(c, "token quota is not configured")
			return
		}
		if !tok.Unlimited && tok.UsedQuota >= tok.Quota {
			abortAuth(c, "quota exhausted")
			return
		}

		c.Set("token", tok)
		c.Next()
	}
}

func abortAuth(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"message": msg, "type": "authentication_error"},
	})
	c.Abort()
}
