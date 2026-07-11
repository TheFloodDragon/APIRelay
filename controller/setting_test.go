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

func TestBillingConfigEndpointsValidateAndPersist(t *testing.T) {
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatal(err)
	}
	model.DB.Exec("DELETE FROM settings")

	bad := performChannelRequest(t, http.MethodPut, "/api/settings/billing", `{"cache_write_multiplier":-1,"cache_read_multiplier":0.1}`, UpdateBillingConfig)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid PUT status = %d, body=%s", bad.Code, bad.Body.String())
	}

	put := performChannelRequest(t, http.MethodPut, "/api/settings/billing", `{"cache_write_multiplier":1.5,"cache_read_multiplier":0.2}`, UpdateBillingConfig)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", put.Code, put.Body.String())
	}
	var response struct {
		Success bool                `json:"success"`
		Data    model.BillingConfig `json:"data"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || response.Data.CacheWriteMultiplier != 1.5 || response.Data.CacheReadMultiplier != 0.2 {
		t.Fatalf("PUT response = %+v", response)
	}

	get := performChannelRequest(t, http.MethodGet, "/api/settings/billing", "", GetBillingConfig)
	if get.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", get.Code, get.Body.String())
	}
}
