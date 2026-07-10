package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apirelay/apirelay/common/config"
)

// TestSetup_Smoke 验证路由装配成功（含可信代理设置），并可响应健康检查。
func TestSetup_Smoke(t *testing.T) {
	cfg := config.Default()
	cfg.Server.TrustedProxies = []string{"127.0.0.1", "10.0.0.0/8"}

	r, err := Setup(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rec.Code)
	}
}

// TestSetup_InvalidTrustedProxy 非法 CIDR 应导致装配失败。
func TestSetup_InvalidTrustedProxy(t *testing.T) {
	cfg := config.Default()
	cfg.Server.TrustedProxies = []string{"not-a-cidr/xx"}
	if _, err := Setup(cfg); err == nil {
		t.Fatal("expected setup to fail on invalid trusted proxy")
	}
}
