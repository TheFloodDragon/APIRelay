package controller

import (
	"net/http"
	"strings"

	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AdminLogin POST /api/auth/login
func AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}

	ip := c.ClientIP()
	if locked, retryAfter := authLoginLimiter.IsLocked(ip, req.Username); locked {
		logger.FromContext(c.Request.Context()).Warn("auth.login_locked",
			zap.String("username", req.Username),
			zap.String("ip", ip),
			zap.Duration("retry_after", retryAfter),
		)
		c.Header("Retry-After", retryAfter.String())
		fail(c, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
		return
	}

	u, err := model.GetUserByUsername(req.Username)
	if err != nil || !u.CheckPassword(req.Password) {
		locked, retryAfter := authLoginLimiter.RecordFailure(ip, req.Username)
		fields := []zap.Field{
			zap.String("username", req.Username),
			zap.String("ip", ip),
			zap.Bool("locked", locked),
		}
		if locked {
			fields = append(fields, zap.Duration("retry_after", retryAfter))
		}
		logger.FromContext(c.Request.Context()).Warn("auth.login_failed", fields...)
		if locked {
			c.Header("Retry-After", retryAfter.String())
			fail(c, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
			return
		}
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	authLoginLimiter.Reset(ip, req.Username)

	token, err := signSessionToken(u)
	if err != nil {
		fail(c, http.StatusInternalServerError, "创建会话失败")
		return
	}

	ok(c, gin.H{"token": token, "username": u.Username, "user_id": u.Id})
}

// AdminLogout POST /api/auth/logout
func AdminLogout(c *gin.Context) {
	if jti := c.GetString("session_jti"); jti != "" {
		revokeSession(jti, c.GetInt64("session_exp"))
	}
	ok(c, gin.H{"ok": true})
}

// CurrentUser GET /api/auth/me
func CurrentUser(c *gin.Context) {
	ok(c, gin.H{
		"user_id":  c.GetInt("user_id"),
		"username": c.GetString("username"),
	})
}

// AdminAuth 保护 /api 管理接口的中间件。
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := sessionTokenFromRequest(c)
		if token == "" {
			fail(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}
		payload, err := verifySessionToken(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, "会话已过期")
			c.Abort()
			return
		}
		c.Set("user_id", payload.UserID)
		c.Set("username", payload.Username)
		c.Set("session_jti", payload.JTI)
		c.Set("session_exp", payload.Exp)
		c.Next()
	}
}

func sessionTokenFromRequest(c *gin.Context) string {
	if h := strings.TrimSpace(c.GetHeader("Authorization")); h != "" {
		if len(h) >= 7 && strings.EqualFold(h[:7], "Bearer ") {
			return strings.TrimSpace(h[7:])
		}
		return h
	}
	return c.GetHeader("X-Session-Token")
}
