package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
