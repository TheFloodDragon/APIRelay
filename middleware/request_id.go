package middleware

import (
	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/common/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestID 为每个请求生成/透传 request_id，并注入带字段的 logger 到 context。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-Id")
		if rid == "" {
			rid = common.NewRequestID()
		}
		c.Set(logger.RequestIDKey, rid)
		c.Writer.Header().Set("X-Request-Id", rid)

		l := logger.L().With(zap.String("request_id", rid))
		ctx := logger.WithContext(c.Request.Context(), l)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
