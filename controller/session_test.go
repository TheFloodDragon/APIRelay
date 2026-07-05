package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
)

func TestSessionTokenSignedAndVerified(t *testing.T) {
	if err := InitAuth(config.AuthConfig{SessionSecret: "test-secret", AllowInsecureDefaultAdmin: true}); err != nil {
		t.Fatal(err)
	}
	tok, err := signSessionToken(&model.User{Id: 7, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, "sess-") {
		t.Fatalf("token prefix = %q", tok)
	}
	payload, err := verifySessionToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if payload.UserID != 7 || payload.Username != "admin" || payload.JTI == "" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestSessionSecretChangeInvalidatesToken(t *testing.T) {
	if err := InitAuth(config.AuthConfig{SessionSecret: "old-secret", AllowInsecureDefaultAdmin: true}); err != nil {
		t.Fatal(err)
	}
	tok, err := signSessionToken(&model.User{Id: 1, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := InitAuth(config.AuthConfig{SessionSecret: "new-secret", AllowInsecureDefaultAdmin: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := verifySessionToken(tok); err == nil {
		t.Fatal("old token should be invalid after secret change")
	}
}

func TestSessionRevoke(t *testing.T) {
	if err := InitAuth(config.AuthConfig{SessionSecret: "test-secret", AllowInsecureDefaultAdmin: true}); err != nil {
		t.Fatal(err)
	}
	tok, err := signSessionToken(&model.User{Id: 1, Username: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := verifySessionToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	revokeSession(payload.JTI, payload.Exp)
	if _, err := verifySessionToken(tok); err == nil {
		t.Fatal("revoked token should be rejected")
	}
}

func TestInitAuthRejectsMissingSecretWhenInsecureDisabled(t *testing.T) {
	if err := InitAuth(config.AuthConfig{AllowInsecureDefaultAdmin: false}); err == nil {
		t.Fatal("missing session secret should fail when insecure mode is disabled")
	}
}

func TestLoginLimiter(t *testing.T) {
	l := newLoginLimiter(2, time.Minute, time.Minute)
	if locked, _ := l.RecordFailure("127.0.0.1", "Admin"); locked {
		t.Fatal("first failure should not lock")
	}
	if locked, _ := l.IsLocked("127.0.0.1", "admin"); locked {
		t.Fatal("should not be locked after one failure")
	}
	if locked, _ := l.RecordFailure("127.0.0.1", "admin"); !locked {
		t.Fatal("second failure should lock")
	}
	if locked, _ := l.IsLocked("127.0.0.1", "ADMIN"); !locked {
		t.Fatal("lock key should be case-insensitive by username")
	}
	l.Reset("127.0.0.1", "admin")
	if locked, _ := l.IsLocked("127.0.0.1", "admin"); locked {
		t.Fatal("reset should clear lock")
	}
}
