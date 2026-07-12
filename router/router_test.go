package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/controller"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay"
	"github.com/apirelay/apirelay/relay/circuitbreaker"
	"github.com/gin-gonic/gin"
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

func TestResetHealthRouteRestoresDispatchCandidate(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file:router-breaker-reset?mode=memory&cache=shared"}); err != nil {
		t.Fatal(err)
	}
	channel := &model.Channel{
		Name: "route-reset", Status: model.ChannelStatusEnabled, Group: "default",
		Models: "route-reset-model", Weight: 1, CooldownUntil: time.Now().Add(time.Hour).UnixMilli(),
	}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ChannelHealth{ChannelId: channel.Id, CircuitState: model.CircuitOpen, CircuitOpenedAt: ptrTime(time.Now())}).Error; err != nil {
		t.Fatal(err)
	}

	mgr := circuitbreaker.GetManager()
	breaker := mgr.GetBreaker(channel.Id)
	breaker.RecordFailure("keep open")
	candidates := []model.ChannelCandidate{{Channel: channel, Priority: channel.Priority, Weight: channel.Weight}}
	if got := relay.SelectFromCandidates(candidates, nil, time.Now().UnixMilli()); got != nil {
		t.Fatalf("open/cooldown candidate should be unavailable: %+v", got)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/channels/:id/health/reset", controller.ResetChannelHealth)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/"+strconv.Itoa(channel.Id)+"/health/reset", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, body = %s", rec.Code, rec.Body.String())
	}

	health, err := model.GetChannelHealth(channel.Id)
	if err != nil {
		t.Fatal(err)
	}
	var refreshed model.Channel
	if err := model.DB.First(&refreshed, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	if health.CircuitState != model.CircuitClosed || refreshed.CooldownUntil != 0 {
		t.Fatalf("health=%+v cooldown=%d", health, refreshed.CooldownUntil)
	}
	candidates[0].Channel = &refreshed
	if got := relay.SelectFromCandidates(candidates, nil, time.Now().UnixMilli()); got == nil || got.Id != channel.Id {
		t.Fatalf("reset candidate was not restored: %+v", got)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
