package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
	"github.com/gin-gonic/gin"
)

// API 契约测试。
//
// 路由清单已有 50+ 条，手工核对「有没有漏加鉴权」既不可靠也不可重复。
// 这里把真实路由装配起来，用 gin 的路由树反射出实际注册的端点，
// 与显式声明的期望清单逐条比对：
//   - 新增路由但忘记登记 → 测试失败，强制作者声明它的鉴权语义
//   - 删除路由但清单未更新 → 测试失败
//   - 本该受保护的端点未鉴权 → 测试失败
//
// 这样"漏加鉴权"从一个需要人眼发现的问题，变成一个必然失败的测试。

// authRequirement 描述一个端点的鉴权语义。
type authRequirement int

const (
	// authPublic 无需任何凭据（健康检查、登录、静态资源）。
	authPublic authRequirement = iota
	// authSession 需要管理后台会话（/api/*）。
	authSession
	// authToken 需要 relay 令牌（/v1/*）。
	authToken
)

type routeContract struct {
	method string
	path   string
	auth   authRequirement
}

// expectedRoutes 是全部对外端点的显式清单。
// 新增路由时必须在此登记，否则契约测试会失败。
var expectedRoutes = []routeContract{
	// 公开端点
	{http.MethodGet, "/healthz", authPublic},
	{http.MethodPost, "/api/auth/login", authPublic},

	// Relay 端点：令牌鉴权
	{http.MethodGet, "/v1/models", authToken},
	{http.MethodPost, "/v1/chat/completions", authToken},
	{http.MethodPost, "/v1/messages", authToken},
	{http.MethodPost, "/v1/responses", authToken},

	// 管理后台：会话鉴权
	{http.MethodPost, "/api/auth/logout", authSession},
	{http.MethodGet, "/api/auth/me", authSession},
	{http.MethodGet, "/api/dashboard", authSession},

	{http.MethodGet, "/api/channel-types", authSession},
	{http.MethodGet, "/api/protocols", authSession},
	{http.MethodGet, "/api/channels", authSession},
	{http.MethodPost, "/api/channels", authSession},
	{http.MethodPost, "/api/channels/reorder", authSession},
	{http.MethodPost, "/api/channels/bulk-delete", authSession},
	{http.MethodPut, "/api/channels/:id", authSession},
	{http.MethodPatch, "/api/channels/:id/status", authSession},
	{http.MethodPatch, "/api/channels/:id/models", authSession},
	{http.MethodDelete, "/api/channels/:id/models", authSession},
	{http.MethodDelete, "/api/channels/:id", authSession},
	{http.MethodGet, "/api/channels/:id/models", authSession},
	{http.MethodPost, "/api/channels/probe-models", authSession},
	{http.MethodPost, "/api/channels/:id/test", authSession},
	{http.MethodPost, "/api/channels/:id/test-all", authSession},
	{http.MethodPost, "/api/channels/test", authSession},
	{http.MethodPost, "/api/channels/test-batch", authSession},
	{http.MethodGet, "/api/channels/:id/health", authSession},
	{http.MethodPost, "/api/channels/:id/health/reset", authSession},

	{http.MethodGet, "/api/models", authSession},

	{http.MethodGet, "/api/settings/protocol-rules", authSession},
	{http.MethodPut, "/api/settings/protocol-rules", authSession},
	{http.MethodGet, "/api/settings/model-prices", authSession},
	{http.MethodPut, "/api/settings/model-prices", authSession},
	{http.MethodGet, "/api/settings/billing", authSession},
	{http.MethodPut, "/api/settings/billing", authSession},
	{http.MethodGet, "/api/settings/model-health", authSession},
	{http.MethodPut, "/api/settings/model-health", authSession},
	{http.MethodGet, "/api/settings/logging", authSession},
	{http.MethodPut, "/api/settings/logging", authSession},
	{http.MethodGet, "/api/settings/network", authSession},
	{http.MethodPut, "/api/settings/network", authSession},
	{http.MethodPost, "/api/settings/network/test", authSession},
	{http.MethodGet, "/api/settings/test-prompt", authSession},
	{http.MethodPut, "/api/settings/test-prompt", authSession},
	{http.MethodGet, "/api/settings/circuit-breaker", authSession},
	{http.MethodPut, "/api/settings/circuit-breaker", authSession},
	{http.MethodGet, "/api/settings/health-stats", authSession},

	{http.MethodGet, "/api/tokens", authSession},
	{http.MethodPost, "/api/tokens", authSession},
	{http.MethodDelete, "/api/tokens/:id", authSession},

	{http.MethodGet, "/api/logs", authSession},
	{http.MethodGet, "/api/logs/export", authSession},
	{http.MethodGet, "/api/logs/:id", authSession},
}

func routeKey(method, path string) string {
	return method + " " + path
}

func setupContractRouter(t *testing.T) *gin.Engine {
	t.Helper()
	// 必须初始化数据库：令牌校验会查库，DB 未就绪时中间件按「依赖故障」返回 500，
	// 那样就测不到真正的鉴权分支了。
	if err := model.InitDB(&config.DatabaseConfig{
		Driver: "sqlite", DSN: "file:router-contract?mode=memory&cache=shared",
	}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	r, err := Setup(config.Default())
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}
	return r
}

// 实际注册的路由与期望清单必须完全一致。
func TestRouteContractMatchesRegisteredRoutes(t *testing.T) {
	r := setupContractRouter(t)

	registered := map[string]struct{}{}
	for _, info := range r.Routes() {
		// NoRoute 处理的 SPA fallback 不出现在 Routes() 里，无需排除。
		registered[routeKey(info.Method, info.Path)] = struct{}{}
	}

	expected := map[string]struct{}{}
	for _, item := range expectedRoutes {
		expected[routeKey(item.method, item.path)] = struct{}{}
	}

	var missing, unexpected []string
	for key := range expected {
		if _, ok := registered[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range registered {
		if _, ok := expected[key]; !ok {
			unexpected = append(unexpected, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)

	if len(missing) > 0 {
		t.Errorf("清单中声明但未注册的路由:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(unexpected) > 0 {
		t.Errorf("已注册但未在 expectedRoutes 登记的路由（新增端点必须声明其鉴权语义）:\n  %s",
			strings.Join(unexpected, "\n  "))
	}
}

// concreteePath 把带参数的路由模板替换成可请求的具体路径。
func concretePath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "1"
		}
	}
	return strings.Join(segments, "/")
}

// 所有 /api/* 端点（除登录）在无凭据时必须返回 401。
func TestAdminRoutesRejectUnauthenticatedRequests(t *testing.T) {
	r := setupContractRouter(t)

	for _, item := range expectedRoutes {
		if item.auth != authSession {
			continue
		}
		t.Run(routeKey(item.method, item.path), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(item.method, concretePath(item.path), strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (endpoint must be behind session auth); body = %s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

// 所有 /v1/* 端点在无令牌时必须返回 401。
func TestRelayRoutesRejectRequestsWithoutToken(t *testing.T) {
	r := setupContractRouter(t)

	for _, item := range expectedRoutes {
		if item.auth != authToken {
			continue
		}
		t.Run(routeKey(item.method, item.path), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(item.method, concretePath(item.path), strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (relay endpoint must require a token); body = %s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

// 无效令牌同样必须被拒，避免"只要带了 Authorization 头就放行"这类退化。
func TestRelayRoutesRejectInvalidToken(t *testing.T) {
	r := setupContractRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"m","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-definitely-not-a-real-token")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unknown token; body = %s", rec.Code, rec.Body.String())
	}
}

// 公开端点必须可在无凭据时访问（否则登录流程会死锁）。
func TestPublicRoutesAreReachableWithoutCredentials(t *testing.T) {
	r := setupContractRouter(t)

	t.Run("healthz", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("login", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
			strings.NewReader(`{"username":"nobody","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		// 凭据错误应返回 4xx，但绝不能是 401 之外的鉴权拦截或 404。
		if rec.Code == http.StatusNotFound {
			t.Fatalf("login endpoint must be registered; status = %d", rec.Code)
		}
	})
}

// SPA fallback 只接管 GET，且不得吞掉未知的 API 路径。
func TestUnknownAPIPathsReturnJSONNotFound(t *testing.T) {
	r := setupContractRouter(t)

	cases := []string{"/api/does-not-exist", "/v1/does-not-exist", "/healthz/nested"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(body, "success") {
				t.Fatalf("expected a JSON error body, got %q", body)
			}
		})
	}
}

// 非 GET 的未知路径不应被 SPA fallback 接管成 200 HTML。
func TestUnknownNonGETPathsAreNotServedBySPAFallback(t *testing.T) {
	r := setupContractRouter(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(method, "/definitely-not-a-route", nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
		})
	}
}

// 契约清单本身不能有重复条目，否则计数与断言会失真。
func TestExpectedRoutesHaveNoDuplicates(t *testing.T) {
	seen := map[string]struct{}{}
	for _, item := range expectedRoutes {
		key := routeKey(item.method, item.path)
		if _, dup := seen[key]; dup {
			t.Errorf("duplicate entry in expectedRoutes: %s", key)
		}
		seen[key] = struct{}{}
	}
}

// 防止有人把 relay 端点误登记为公开：/v1 与 /api 下不应存在 authPublic 条目（除登录）。
func TestNoUnexpectedPublicEndpoints(t *testing.T) {
	allowedPublic := map[string]struct{}{
		routeKey(http.MethodGet, "/healthz"):         {},
		routeKey(http.MethodPost, "/api/auth/login"): {},
	}
	for _, item := range expectedRoutes {
		if item.auth != authPublic {
			continue
		}
		key := routeKey(item.method, item.path)
		if _, ok := allowedPublic[key]; !ok {
			t.Errorf("端点 %s 被标为公开，但公开端点必须显式加入白名单并评估风险", key)
		}
	}
}

func init() {
	// 让契约测试的失败信息更易读。
	gin.SetMode(gin.TestMode)
	_ = fmt.Sprint
}
