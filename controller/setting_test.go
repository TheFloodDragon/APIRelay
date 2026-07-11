package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
)

func TestModelHealthConfigEndpointsNormalizeAndPersist(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	model.DB.Exec("DELETE FROM settings")

	put := performChannelRequest(t, http.MethodPut, "/api/settings/model-health", `{
		"recent_count":20000,
		"window_hours":0,
		"healthy_threshold":80,
		"warning_threshold":90
	}`, UpdateModelHealthConfig)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", put.Code, put.Body.String())
	}
	var putResponse struct {
		Success bool                    `json:"success"`
		Data    model.ModelHealthConfig `json:"data"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &putResponse); err != nil {
		t.Fatal(err)
	}
	if !putResponse.Success || putResponse.Data.RecentCount != 10000 || putResponse.Data.WindowHours != 24 || putResponse.Data.HealthyThreshold != 80 || putResponse.Data.WarningThreshold != 80 {
		t.Fatalf("PUT response = %+v", putResponse)
	}

	get := performChannelRequest(t, http.MethodGet, "/api/settings/model-health", "", GetModelHealthConfig)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", get.Code, get.Body.String())
	}
	var getResponse struct {
		Success bool                    `json:"success"`
		Data    model.ModelHealthConfig `json:"data"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &getResponse); err != nil {
		t.Fatal(err)
	}
	if !getResponse.Success || getResponse.Data != putResponse.Data {
		t.Fatalf("GET response = %+v, want %+v", getResponse, putResponse.Data)
	}
}
