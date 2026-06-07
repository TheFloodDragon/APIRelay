package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yourusername/apirelay/pkg/config"
)

// CORSMiddleware CORS中间件
func CORSMiddleware() gin.HandlerFunc {
	cfg := config.GlobalConfig.CORS

	corsConfig := cors.Config{
		AllowAllOrigins:  false,
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     cfg.AllowMethods,
		AllowHeaders:     cfg.AllowHeaders,
		AllowCredentials: true,
	}

	// 如果允许所有源
	if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
		corsConfig.AllowAllOrigins = true
	}

	return cors.New(corsConfig)
}
