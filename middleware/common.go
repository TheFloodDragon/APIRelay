package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/apirelay/apirelay/common/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery 捕获 panic，返回 500 并记录堆栈。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.FromContext(c.Request.Context()).Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": gin.H{"message": "internal server error", "type": "internal_error"},
					})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

// AccessLog 记录每个请求的访问日志（运行日志）。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.FromContext(c.Request.Context()).Info("http.access",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		)
	}
}

// CORS 按 allowlist 允许跨域；allowedOrigins 为空时不主动允许任何 Origin。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-Id, X-Session-Token, x-api-key, anthropic-version")
		h.Set("Access-Control-Max-Age", "86400")

		switch {
		case allowAll:
			h.Set("Access-Control-Allow-Origin", "*")
		case origin != "":
			if _, ok := allowed[origin]; ok {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodySizeLimit 限制请求体大小。超限错误由实际读取方识别为 *http.MaxBytesError。
func BodySizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
