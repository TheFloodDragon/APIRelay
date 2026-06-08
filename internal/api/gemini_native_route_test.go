//go:build cgo

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGeminiCountTokensNativeRouteForwardsToUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamKey string
	var upstreamBody map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.RequestURI()
		upstreamKey = r.Header.Get("x-goog-api-key")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalTokens":7}`))
	}))
	defer upstream.Close()

	db := newGeminiNativeRouteTestDB(t)
	channel := model.Channel{
		Name:         "gemini-test",
		Type:         "gemini",
		APIKey:       "AIza-upstream",
		BaseURL:      upstream.URL + "/v1beta",
		Models:       model.JSONStringList{"gemini-pro"},
		Enabled:      true,
		Weight:       1,
		HealthStatus: "healthy",
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.Model{Name: "gemini-pro", DisplayName: "gemini-pro", ChannelID: channel.ID, Enabled: true}).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := db.Create(&model.APIKey{Key: "relay-key", Enabled: true}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	cfg := geminiNativeRouteTestConfig()
	router := SetupRouter(db, cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:countTokens", strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
	req.Header.Set("x-goog-api-key", "relay-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-pro:countTokens" {
		t.Fatalf("upstream path = %q, want /v1beta/models/gemini-pro:countTokens", upstreamPath)
	}
	if upstreamKey != "AIza-upstream" {
		t.Fatalf("upstream x-goog-api-key = %q, want AIza-upstream", upstreamKey)
	}
	contents, ok := upstreamBody["contents"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("upstream contents = %#v, want one item", upstreamBody["contents"])
	}
	if resp.Body.String() != `{"totalTokens":7}` {
		t.Fatalf("response body = %s, want countTokens payload", resp.Body.String())
	}
}

func TestGeminiNamespaceModelRouteReturnsModelMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newGeminiNativeRouteTestDB(t)
	channel := model.Channel{
		Name:         "gemini-test",
		Type:         "gemini",
		APIKey:       "AIza-upstream",
		Models:       model.JSONStringList{"gemini-pro"},
		Enabled:      true,
		Weight:       1,
		HealthStatus: "healthy",
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.Model{Name: "gemini-pro", DisplayName: "public-gemini", ChannelID: channel.ID, Enabled: true}).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := db.Create(&model.APIKey{Key: "relay-key", Enabled: true}).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}

	cfg := geminiNativeRouteTestConfig()
	router := SetupRouter(db, cfg)
	req := httptest.NewRequest(http.MethodGet, "/gemini/v1beta/models/public-gemini", nil)
	req.Header.Set("x-goog-api-key", "relay-key")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["name"] != "models/public-gemini" {
		t.Fatalf("name = %v, want models/public-gemini", payload["name"])
	}
}

func geminiNativeRouteTestConfig() *config.Config {
	cfg := &config.Config{Server: config.ServerConfig{Mode: gin.TestMode}}
	config.GlobalConfig = cfg
	return cfg
}

func newGeminiNativeRouteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Channel{}, &model.Model{}, &model.APIKey{}, &model.RequestLog{}, &model.SystemConfig{}, &model.ModelTestLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}
