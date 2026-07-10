package middleware

import (
	"errors"
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
			// 区分「令牌不存在」（鉴权失败，401）与底层 DB 错误（如 SQLite busy，500），
			// 避免把临时性数据库故障误报为无效 API Key。
			if errors.Is(err, model.ErrTokenNotFound) {
				abortAuth(c, "invalid api key")
			} else {
				abortInternal(c)
			}
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

// abortInternal 用于底层依赖（如数据库）故障时返回 500，避免误报鉴权失败。
func abortInternal(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{"message": "internal server error", "type": "internal_error"},
	})
	c.Abort()
}
