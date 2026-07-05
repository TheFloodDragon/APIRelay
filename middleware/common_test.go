package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://console.example"}))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("Origin", "https://console.example")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://console.example" {
		t.Fatalf("allowed origin header = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin should not be allowed, got %q", got)
	}
}

func TestCORSPreflightIncludesRequiredHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"*"}))

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	headers := rec.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"x-api-key", "X-Session-Token", "anthropic-version"} {
		if !strings.Contains(headers, want) {
			t.Fatalf("allow headers %q missing %q", headers, want)
		}
	}
}

func TestBodySizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodySizeLimit(4))
	r.POST("/body", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/body", strings.NewReader("12345"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", rec.Code)
	}
}
