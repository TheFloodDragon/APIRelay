package middleware

import (
	"strings"
	"time"

	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var defaultCORSMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}

var defaultCORSHeaders = []string{
	"Accept",
	"Authorization",
	"Content-Type",
	"Origin",
	"X-Requested-With",
	"X-Request-ID",
	"X-Request-Id",
	"Request-ID",
	"X-API-Key",
	"X-Api-Key",
	"X-Goog-Api-Key",
	"X-Goog-Api-Client",
	"Anthropic-Version",
	"Anthropic-Beta",
	"OpenAI-Organization",
	"OpenAI-Project",
	"OpenAI-Beta",
}

// CORSMiddleware CORS中间件
func CORSMiddleware() gin.HandlerFunc {
	cfg := config.GlobalConfig.CORS

	corsConfig := cors.Config{
		AllowAllOrigins:  false,
		AllowOrigins:     normalizeCORSOrigins(cfg.AllowOrigins),
		AllowMethods:     normalizeCORSMethods(cfg.AllowMethods),
		AllowHeaders:     normalizeCORSHeaders(cfg.AllowHeaders),
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	// 如果允许所有源，不能同时设置 AllowOrigins；同时通配符来源不能携带凭据。
	if len(corsConfig.AllowOrigins) == 1 && corsConfig.AllowOrigins[0] == "*" {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowOrigins = nil
		corsConfig.AllowCredentials = false
	}

	return cors.New(corsConfig)
}

func normalizeCORSOrigins(origins []string) []string {
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}

func normalizeCORSMethods(methods []string) []string {
	if len(methods) == 0 {
		return defaultCORSMethods
	}
	return methods
}

func normalizeCORSHeaders(headers []string) []string {
	if len(headers) == 0 || containsCORSWildcard(headers) {
		return defaultCORSHeaders
	}
	return headers
}

func containsCORSWildcard(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "*" {
			return true
		}
	}
	return false
}
