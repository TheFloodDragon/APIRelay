package middleware

import (
	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	// 如果允许所有源，不能同时设置 AllowOrigins；同时通配符来源不能携带凭据。
	if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowOrigins = nil
		corsConfig.AllowCredentials = false
	}

	return cors.New(corsConfig)
}
