package common

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// NewRequestID 生成一个 32 位十六进制的请求 ID。
func NewRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b)
}

// NewToken 生成一个带前缀的随机令牌（用于对外 API Key）。
func NewToken(prefix string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return prefix + "error"
	}
	return prefix + hex.EncodeToString(b)
}

// TrimBearer 去除 Authorization 头中的 "Bearer " 前缀。
func TrimBearer(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "Bearer ") {
		return strings.TrimSpace(s[7:])
	}
	return s
}

// MaskSecret 返回适合日志展示的密钥脱敏值。
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
