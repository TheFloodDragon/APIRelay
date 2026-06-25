package controller

import (
	"net/http"
	"sync"
	"time"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

// session 管理后台会话（内存存储，简易实现）。
type session struct {
	userID    int
	username  string
	expiresAt time.Time
}

var (
	sessionMu    sync.RWMutex
	sessionStore = map[string]session{}
)

const sessionTTL = 24 * time.Hour

// AdminLogin POST /api/auth/login
func AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	u, err := model.GetUserByUsername(req.Username)
	if err != nil || !u.CheckPassword(req.Password) {
		fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token := common.NewToken("sess-")
	sessionMu.Lock()
	sessionStore[token] = session{userID: u.Id, username: u.Username, expiresAt: time.Now().Add(sessionTTL)}
	sessionMu.Unlock()

	ok(c, gin.H{"token": token, "username": u.Username, "user_id": u.Id})
}

// AdminLogout POST /api/auth/logout
func AdminLogout(c *gin.Context) {
	token := sessionTokenFromRequest(c)
	if token != "" {
		sessionMu.Lock()
		delete(sessionStore, token)
		sessionMu.Unlock()
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
		sessionMu.RLock()
		s, exists := sessionStore[token]
		sessionMu.RUnlock()
		if !exists || time.Now().After(s.expiresAt) {
			if exists {
				sessionMu.Lock()
				delete(sessionStore, token)
				sessionMu.Unlock()
			}
			fail(c, http.StatusUnauthorized, "会话已过期")
			c.Abort()
			return
		}
		c.Set("user_id", s.userID)
		c.Set("username", s.username)
		c.Next()
	}
}

func sessionTokenFromRequest(c *gin.Context) string {
	if h := common.TrimBearer(c.GetHeader("Authorization")); h != "" {
		return h
	}
	return c.GetHeader("X-Session-Token")
}
