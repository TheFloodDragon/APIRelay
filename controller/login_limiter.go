package controller

import (
	"strings"
	"sync"
	"time"

	"github.com/apirelay/apirelay/common/config"
)

type loginLimiter struct {
	mu          sync.Mutex
	records     map[string]*loginFailureRecord
	maxFailures int
	window      time.Duration
	lockout     time.Duration
}

type loginFailureRecord struct {
	failures        int
	windowStartedAt time.Time
	lockedUntil     time.Time
}

var authLoginLimiter = newLoginLimiter(
	config.DefaultLoginMaxFailures,
	time.Duration(config.DefaultLoginFailureWindowSeconds)*time.Second,
	time.Duration(config.DefaultLoginLockoutSeconds)*time.Second,
)

func newLoginLimiter(maxFailures int, window, lockout time.Duration) *loginLimiter {
	if maxFailures <= 0 {
		maxFailures = config.DefaultLoginMaxFailures
	}
	if window <= 0 {
		window = time.Duration(config.DefaultLoginFailureWindowSeconds) * time.Second
	}
	if lockout <= 0 {
		lockout = time.Duration(config.DefaultLoginLockoutSeconds) * time.Second
	}
	return &loginLimiter{
		records:     make(map[string]*loginFailureRecord),
		maxFailures: maxFailures,
		window:      window,
		lockout:     lockout,
	}
}

func (l *loginLimiter) IsLocked(ip, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	key := loginLimiterKey(ip, username)
	rec := l.records[key]
	if rec == nil {
		return false, 0
	}
	if rec.lockedUntil.After(now) {
		return true, time.Until(rec.lockedUntil)
	}
	if !rec.lockedUntil.IsZero() || now.Sub(rec.windowStartedAt) > l.window {
		delete(l.records, key)
	}
	return false, 0
}

func (l *loginLimiter) RecordFailure(ip, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	key := loginLimiterKey(ip, username)
	rec := l.records[key]
	if rec == nil || now.Sub(rec.windowStartedAt) > l.window {
		rec = &loginFailureRecord{windowStartedAt: now}
		l.records[key] = rec
	}
	if rec.lockedUntil.After(now) {
		return true, time.Until(rec.lockedUntil)
	}
	rec.failures++
	if rec.failures >= l.maxFailures {
		rec.lockedUntil = now.Add(l.lockout)
		return true, l.lockout
	}
	return false, 0
}

func (l *loginLimiter) Reset(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, loginLimiterKey(ip, username))
}

func loginLimiterKey(ip, username string) string {
	return strings.TrimSpace(ip) + "\x00" + strings.ToLower(strings.TrimSpace(username))
}
