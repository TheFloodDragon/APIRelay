package middleware

import (
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// APIKeyAuthMiddleware OpenAI兼容API认证中间件
func APIKeyAuthMiddleware(keyRepo *repository.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, formatOK := extractRelayAPIKey(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "缺少 API Key",
					"type":    "invalid_request_error",
				},
			})
			c.Abort()
			return
		}

		if !formatOK {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "API Key 格式错误",
					"type":    "invalid_request_error",
				},
			})
			c.Abort()
			return
		}

		// 管理密钥可用于调试兼容 API 接口
		if token == config.GlobalConfig.Auth.AdminKey {
			c.Next()
			return
		}

		apiKey, err := keyRepo.GetByKey(token)
		if err != nil {
			// 如果数据库中没有任何 API Key，允许管理密钥外的请求失败，避免裸奔
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"message": "无效的 API Key",
						"type":    "invalid_request_error",
					},
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "API Key 验证失败",
					"type":    "internal_error",
				},
			})
			c.Abort()
			return
		}

		_ = keyRepo.UpdateLastUsed(apiKey.ID)
		c.Set("api_key", apiKey)
		c.Set("api_key_id", apiKey.ID)
		c.Next()
	}
}

func extractRelayAPIKey(c *gin.Context) (string, bool) {
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return "", false
		}
		return strings.TrimSpace(parts[1]), true
	}

	// Anthropic 官方 SDK 默认使用 x-api-key。
	if token := strings.TrimSpace(c.GetHeader("x-api-key")); token != "" {
		return token, true
	}

	// Gemini 官方 SDK/REST 常用 x-goog-api-key 或 ?key=。
	if token := strings.TrimSpace(c.GetHeader("x-goog-api-key")); token != "" {
		return token, true
	}
	if token := strings.TrimSpace(c.Query("key")); token != "" {
		return token, true
	}

	return "", true
}

// GetAPIKeyFromContext 从上下文获取API Key
func GetAPIKeyFromContext(c *gin.Context) *model.APIKey {
	value, ok := c.Get("api_key")
	if !ok {
		return nil
	}
	apiKey, ok := value.(*model.APIKey)
	if !ok {
		return nil
	}
	return apiKey
}
