package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggerMiddleware 日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		log.Printf("%s method=%s path=%s status=%d latency=%s ip=%s",
			accessLogPrefix(path),
			method,
			path,
			statusCode,
			latency,
			c.ClientIP(),
		)
	}
}

func accessLogPrefix(path string) string {
	switch {
	case strings.HasPrefix(path, "/v1/"):
		return "[MODEL-HTTP]"
	case strings.HasPrefix(path, "/api/"):
		return "[API]"
	default:
		return "[WEB]"
	}
}
