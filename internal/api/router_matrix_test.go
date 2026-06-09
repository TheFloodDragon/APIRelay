package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRelayRouteMatrixIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	setupRelayRoutes(r, nil, nil)
	setupStatusRoute(r)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/models"},
		{http.MethodGet, "/v1/models"},
		{http.MethodPost, "/v1/messages"},
		{http.MethodPost, "/claude/v1/messages"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/v1/chat/completions"},
		{http.MethodPost, "/codex/v1/chat/completions"},
		{http.MethodPost, "/responses"},
		{http.MethodPost, "/v1/responses"},
		{http.MethodPost, "/v1/v1/responses"},
		{http.MethodPost, "/codex/v1/responses"},
		{http.MethodPost, "/responses/compact"},
		{http.MethodPost, "/v1/responses/compact"},
		{http.MethodPost, "/v1beta/models/gemini-pro:generateContent"},
		{http.MethodGet, "/gemini/v1beta/models/gemini-pro"},
		{http.MethodPost, "/gemini/v1/models/gemini-pro:generateContent"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			resp := httptest.NewRecorder()

			r.ServeHTTP(resp, req)

			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s; want route-level auth rejection", resp.Code, resp.Body.String())
			}
			body := resp.Body.String()
			if strings.Contains(strings.ToLower(body), "<html") || strings.Contains(body, "404 page not found") {
				t.Fatalf("route fell through to SPA/Gin 404 body: %s", body)
			}
		})
	}
}

func TestRelayHealthAndStatusRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	setupRelayRoutes(r, nil, nil)
	setupStatusRoute(r)

	for _, path := range []string{"/health", "/status"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			resp := httptest.NewRecorder()

			r.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s; want status route", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), `"status":"running"`) {
				t.Fatalf("body = %s, want running status", resp.Body.String())
			}
		})
	}
}
