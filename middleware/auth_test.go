package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"

	"github.com/gin-gonic/gin"
)

func setupAuthTestDB(t *testing.T) {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM tokens")
}

func newAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TokenAuth())
	r.GET("/v1/models", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func doAuthReq(r *gin.Engine, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestTokenAuth_ValidToken 有效令牌应放行。
func TestTokenAuth_ValidToken(t *testing.T) {
	setupAuthTestDB(t)
	tok := &model.Token{Name: "ok", Unlimited: true, Status: model.TokenStatusEnabled}
	if err := model.CreateToken(tok, "sk-valid"); err != nil {
		t.Fatalf("create token: %v", err)
	}
	rec := doAuthReq(newAuthRouter(), "sk-valid")
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// TestTokenAuth_NotFoundReturns401 令牌不存在应 401。
func TestTokenAuth_NotFoundReturns401(t *testing.T) {
	setupAuthTestDB(t)
	rec := doAuthReq(newAuthRouter(), "sk-nonexistent")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("not-found status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

// TestTokenAuth_DBErrorReturns500 底层 DB 错误应 500 而非误报 401。
func TestTokenAuth_DBErrorReturns500(t *testing.T) {
	setupAuthTestDB(t)
	// 删除 tokens 表以制造真实 DB 错误（no such table），非 ErrTokenNotFound。
	if err := model.DB.Migrator().DropTable(&model.Token{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	rec := doAuthReq(newAuthRouter(), "sk-anything")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("db-error status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

// TestTokenAuth_MissingKeyReturns401 缺少 API Key 应 401。
func TestTokenAuth_MissingKeyReturns401(t *testing.T) {
	setupAuthTestDB(t)
	rec := doAuthReq(newAuthRouter(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing-key status = %d, want 401", rec.Code)
	}
}
