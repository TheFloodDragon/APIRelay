package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"
)

const sessionTTL = 24 * time.Hour

var (
	authSessionMu     sync.RWMutex
	authSessionSecret []byte
	revokedSessions   = map[string]int64{}

	errInvalidSession = errors.New("invalid session")
	errExpiredSession = errors.New("expired session")
)

type sessionPayload struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
	JTI      string `json:"jti"`
}

// InitAuth 初始化管理后台认证组件。router.Setup 会在注册路由前调用。
func InitAuth(cfg config.AuthConfig) error {
	secret := strings.TrimSpace(cfg.SessionSecret)
	if secret == "" {
		if !cfg.AllowInsecureDefaultAdmin {
			return fmt.Errorf("auth.session_secret is required; set APIRELAY_SESSION_SECRET or auth.session_secret")
		}
		secret = common.NewToken("sess-secret-")
		logger.L().Warn("using temporary session secret; sessions will be invalid after restart")
	}

	authSessionMu.Lock()
	authSessionSecret = []byte(secret)
	revokedSessions = map[string]int64{}
	authSessionMu.Unlock()

	authLoginLimiter = newLoginLimiter(
		cfg.LoginMaxFailures,
		time.Duration(cfg.LoginFailureWindowSeconds)*time.Second,
		time.Duration(cfg.LoginLockoutSeconds)*time.Second,
	)
	return nil
}

func signSessionToken(u *model.User) (string, error) {
	secret := currentSessionSecret()
	if len(secret) == 0 {
		return "", errInvalidSession
	}
	payload := sessionPayload{
		UserID:   u.Id,
		Username: u.Username,
		Exp:      time.Now().Add(sessionTTL).Unix(),
		JTI:      common.NewToken("jti-"),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sigPart := signPayload(payloadPart, secret)
	return "sess-" + payloadPart + "." + sigPart, nil
}

func verifySessionToken(token string) (*sessionPayload, error) {
	secret := currentSessionSecret()
	if len(secret) == 0 {
		return nil, errInvalidSession
	}
	if !strings.HasPrefix(token, "sess-") {
		return nil, errInvalidSession
	}
	parts := strings.Split(strings.TrimPrefix(token, "sess-"), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errInvalidSession
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidSession
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(gotSig, mac.Sum(nil)) {
		return nil, errInvalidSession
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errInvalidSession
	}
	var payload sessionPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errInvalidSession
	}
	if payload.UserID <= 0 || payload.Username == "" || payload.JTI == "" {
		return nil, errInvalidSession
	}
	now := time.Now().Unix()
	if payload.Exp <= now {
		return nil, errExpiredSession
	}
	if isSessionRevoked(payload.JTI, now) {
		return nil, errExpiredSession
	}
	return &payload, nil
}

func revokeSession(jti string, exp int64) {
	if jti == "" {
		return
	}
	if exp <= 0 {
		exp = time.Now().Add(sessionTTL).Unix()
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	cleanupRevokedLocked(time.Now().Unix())
	revokedSessions[jti] = exp
}

func isSessionRevoked(jti string, now int64) bool {
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	cleanupRevokedLocked(now)
	exp, ok := revokedSessions[jti]
	return ok && exp > now
}

func cleanupRevokedLocked(now int64) {
	for jti, exp := range revokedSessions {
		if exp <= now {
			delete(revokedSessions, jti)
		}
	}
}

func currentSessionSecret() []byte {
	authSessionMu.RLock()
	defer authSessionMu.RUnlock()
	if len(authSessionSecret) == 0 {
		return nil
	}
	secret := make([]byte, len(authSessionSecret))
	copy(secret, authSessionSecret)
	return secret
}

func signPayload(payloadPart string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payloadPart))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
