package middleware

import (
	"net/http"
	"strings"

	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 管理API认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 健康检查接口跳过认证
		if c.Request.URL.Path == "/api/system/health" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "缺少认证令牌",
			})
			c.Abort()
			return
		}

		// 检查 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌格式错误",
			})
			c.Abort()
			return
		}

		token := parts[1]
		if token != config.GlobalConfig.Auth.AdminKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌无效",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
