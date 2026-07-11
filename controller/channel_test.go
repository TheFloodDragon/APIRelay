package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
	"github.com/gin-gonic/gin"
)

func performChannelRequest(t *testing.T, method, path, body string, handler gin.HandlerFunc, params ...gin.Param) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = params
	ctx.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)
	return recorder
}

func assertHeaderOverrideFieldError(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Success || !strings.Contains(response.Message, "header_override:") {
		t.Fatalf("response = %+v", response)
	}
}

func TestTemporaryChannelEndpointsRejectInvalidHeaderOverride(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		body    string
		handler gin.HandlerFunc
	}{
		{name: "single test", path: "/api/channels/test", body: `{"key":"k","model":"m","header_override":"[]"}`, handler: TestChannelByConfig},
		{name: "batch test", path: "/api/channels/test-batch", body: `{"key":"k","models":["m"],"header_override":"{\"X-Count\":1}"}`, handler: TestChannelBatchByConfig},
		{name: "model probe", path: "/api/channels/probe-models", body: `{"key":"k","header_override":"{"}`, handler: ProbeModelsByConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performChannelRequest(t, http.MethodPost, tt.path, tt.body, tt.handler)
			assertHeaderOverrideFieldError(t, recorder)
		})
	}
}

func TestCreateChannelRejectsInvalidHeaderOverride(t *testing.T) {
	recorder := performChannelRequest(t, http.MethodPost, "/api/channels", `{"key":"k","header_override":"null"}`, CreateChannel)
	assertHeaderOverrideFieldError(t, recorder)
}

func assertBodyOverrideFieldError(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Success || !strings.Contains(response.Message, "body_override:") {
		t.Fatalf("response = %+v", response)
	}
}

func TestChannelEndpointsRejectInvalidBodyOverride(t *testing.T) {
	// 非对象的 body_override 应被拒绝（数组 / 标量 / 非法 JSON）。
	recorder := performChannelRequest(t, http.MethodPost, "/api/channels", `{"key":"k","body_override":"[]"}`, CreateChannel)
	assertBodyOverrideFieldError(t, recorder)

	recorder = performChannelRequest(t, http.MethodPost, "/api/channels/test", `{"key":"k","model":"m","body_override":"123"}`, TestChannelByConfig)
	assertBodyOverrideFieldError(t, recorder)
}

func TestCreateChannelAcceptsValidBodyOverride(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	recorder := performChannelRequest(t, http.MethodPost, "/api/channels",
		`{"name":"ok","key":"k","models":"m","model_configs":"[{\"name\":\"m\",\"enabled\":true}]","body_override":"{\"reasoning\":{\"effort\":\"high\"}}"}`,
		CreateChannel)
	if recorder.Code != http.StatusOK {
		t.Fatalf("valid body_override should be accepted, status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSavedChannelEndpointsRejectInvalidHeaderOverride(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	model.DB.Exec("DELETE FROM channels")
	model.DB.Exec("DELETE FROM abilities")
	ch := &model.Channel{
		Name: "invalid-headers", Type: 1, Status: model.ChannelStatusEnabled, Key: "k", Group: "default", Weight: 1,
		ModelConfigs: `[{"name":"m","enabled":true}]`, HeaderOverride: `[]`,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(ch.Id)

	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		handler gin.HandlerFunc
	}{
		{name: "update", method: http.MethodPut, path: "/api/channels/" + id, body: `{"key":"k","header_override":"[]"}`, handler: UpdateChannel},
		{name: "single test", method: http.MethodPost, path: "/api/channels/" + id + "/test", body: `{"model":"m"}`, handler: TestChannelModel},
		{name: "batch test", method: http.MethodPost, path: "/api/channels/" + id + "/test-all", body: `{"models":["m"]}`, handler: TestChannelAllModels},
		{name: "model probe", method: http.MethodGet, path: "/api/channels/" + id + "/models", body: "", handler: ProbeChannelModels},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performChannelRequest(t, tt.method, tt.path, tt.body, tt.handler, gin.Param{Key: "id", Value: id})
			assertHeaderOverrideFieldError(t, recorder)
		})
	}
}

func setupControllerHealthTestDB(t *testing.T) *model.Channel {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	model.DB.Exec("DELETE FROM logs")
	model.DB.Exec("DELETE FROM channels")
	model.DB.Exec("DELETE FROM abilities")
	ch := &model.Channel{
		Name: "primary", Type: 1, Status: model.ChannelStatusEnabled, BaseURL: "https://example.test", Key: "k", Group: "default", Weight: 1,
		ModelConfigs: `[{"name":"gpt-4o","enabled":true},{"name":"never-used","enabled":true}]`,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Minute).UnixMilli()
	logs := []*model.Log{
		{RequestId: "ok", Type: model.LogTypeConsume, ChannelId: ch.Id, SrcModel: "gpt-4o", Status: 200, CreatedAt: base + 100},
		{RequestId: "fail", Type: model.LogTypeError, ChannelId: ch.Id, SrcModel: "gpt-4o", Status: 503, Error: "upstream unavailable", CreatedAt: base + 200},
	}
	for _, item := range logs {
		if err := model.CreateLog(item); err != nil {
			t.Fatal(err)
		}
	}
	return ch
}

func TestListChannelsIncludesModelHealth(t *testing.T) {
	setupControllerHealthTestDB(t)
	recorder := performChannelRequest(t, http.MethodGet, "/api/channels", "", ListChannels)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Name        string                            `json:"name"`
			ModelHealth map[string]*model.ModelHealthStat `json:"model_health"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || len(response.Data) != 1 {
		t.Fatalf("response = %+v", response)
	}
	health := response.Data[0].ModelHealth["gpt-4o"]
	if health == nil || health.Total != 2 || health.Success != 1 || health.Failed != 1 || health.LastError != "upstream unavailable" {
		t.Fatalf("gpt-4o health = %+v", health)
	}
	empty := response.Data[0].ModelHealth["never-used"]
	if empty == nil || empty.Total != 0 || empty.Model != "never-used" {
		t.Fatalf("never-used health = %+v", empty)
	}
}

func TestListAggregatedModelsIncludesHealth(t *testing.T) {
	setupControllerHealthTestDB(t)
	recorder := performChannelRequest(t, http.MethodGet, "/api/models", "", ListAggregatedModels)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Name      string                 `json:"name"`
			Health    *model.ModelHealthStat `json:"health"`
			Providers []struct {
				ChannelName string                 `json:"channel_name"`
				Health      *model.ModelHealthStat `json:"health"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || len(response.Data) != 2 {
		t.Fatalf("response = %+v", response)
	}
	var found bool
	for _, item := range response.Data {
		if item.Name != "gpt-4o" {
			continue
		}
		found = true
		if item.Health == nil || item.Health.Total != 2 || item.Health.Success != 1 || item.Health.Failed != 1 {
			t.Fatalf("aggregate health = %+v", item.Health)
		}
		if len(item.Providers) != 1 || item.Providers[0].Health == nil || item.Providers[0].Health.LastError != "upstream unavailable" {
			t.Fatalf("providers = %+v", item.Providers)
		}
	}
	if !found {
		t.Fatal("missing gpt-4o aggregate model")
	}
}
