package model

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	DB.Exec("DELETE FROM logs")
	DB.Exec("DELETE FROM log_payloads")
	DB.Exec("DELETE FROM settings")
	invalidateModelHealthConfigCache()
}

func TestLoggingConfigDefault(t *testing.T) {
	// 重置缓存
	loggingConfigMu.Lock()
	loggingConfigLoaded = false
	loggingConfigCache = nil
	loggingConfigMu.Unlock()

	cfg := GetLoggingConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Enabled {
		t.Error("expected default Enabled=false")
	}
	if len(cfg.SanitizedHeaderKeys) == 0 {
		t.Error("expected default sanitized headers")
	}
	// 验证默认包含敏感 headers
	found := false
	for _, k := range cfg.SanitizedHeaderKeys {
		if k == "Authorization" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Authorization in default sanitized headers")
	}
}

func TestModelHealthConfigDefaultsNormalizationAndPersistence(t *testing.T) {
	setupTestDB(t)

	defaults := GetModelHealthConfig()
	if defaults.RecentCount != 100 || defaults.WindowHours != 24 || defaults.HealthyThreshold != 95 || defaults.WarningThreshold != 70 {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	normalized := NormalizeModelHealthConfig(ModelHealthConfig{
		RecentCount:      20000,
		WindowHours:      -1,
		HealthyThreshold: 65,
		WarningThreshold: 90,
	})
	if normalized.RecentCount != maxModelHealthRecentCount || normalized.WindowHours != 24 {
		t.Fatalf("unexpected normalized limits: %+v", normalized)
	}
	if normalized.HealthyThreshold != 65 || normalized.WarningThreshold != 65 {
		t.Fatalf("unexpected normalized thresholds: %+v", normalized)
	}

	saved, err := SaveModelHealthConfig(ModelHealthConfig{RecentCount: 25, WindowHours: 48, HealthyThreshold: 90, WarningThreshold: 60})
	if err != nil {
		t.Fatalf("save model health config: %v", err)
	}
	if got := GetModelHealthConfig(); got != saved {
		t.Fatalf("persisted config = %+v, want %+v", got, saved)
	}
}

func TestNormalizeLoggingConfigKeepsMandatoryRedaction(t *testing.T) {
	cfg := NormalizeLoggingConfig(LoggingConfig{Enabled: true, SanitizedHeaderKeys: []string{"X-Custom-Secret"}})
	for _, required := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-API-Key", "X-Custom-Secret"} {
		found := false
		for _, key := range cfg.SanitizedHeaderKeys {
			if key == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing required redaction key %q", required)
		}
	}
}

func TestLogPayloadCompression(t *testing.T) {
	setupTestDB(t)

	// 创建一条主日志
	log := &Log{
		RequestId: "test-req-123",
		Type:      LogTypeConsume,
		Status:    200,
	}
	if err := CreateLog(log); err != nil {
		t.Fatal(err)
	}

	// 创建完整日志载荷
	data := &FullLogData{
		ClientRequest:    `{"method":"POST","body":"test request body that should be compressed"}`,
		UpstreamRequest:  `{"url":"https://api.example.com","body":"upstream request"}`,
		UpstreamResponse: `{"status":200,"body":"upstream response with some data"}`,
		ClientResponse:   `{"status":200,"body":"client response"}`,
	}
	if err := CreateLogPayload(log.Id, data); err != nil {
		t.Fatal(err)
	}

	// 读取并验证
	retrieved, err := GetLogPayload(log.Id)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.ClientRequest != data.ClientRequest {
		t.Errorf("client request mismatch: got %q, want %q", retrieved.ClientRequest, data.ClientRequest)
	}
	if retrieved.UpstreamResponse != data.UpstreamResponse {
		t.Errorf("upstream response mismatch")
	}
}

func TestSaveFullLogPayloadSerializesCaptureFields(t *testing.T) {
	setupTestDB(t)
	log := &Log{RequestId: "capture-req", Type: LogTypeError, Status: 502}
	if err := CreateLog(log); err != nil {
		t.Fatal(err)
	}
	capture := struct {
		ClientMethod     string            `json:"client_method"`
		ClientPath       string            `json:"client_path"`
		ClientHeaders    map[string]string `json:"client_headers"`
		ClientBody       []byte            `json:"client_body"`
		UpstreamURL      string            `json:"upstream_url"`
		UpstreamBody     []byte            `json:"upstream_body"`
		ClientRespStatus int               `json:"client_resp_status"`
		ClientRespBody   []byte            `json:"client_resp_body"`
	}{
		ClientMethod: "POST", ClientPath: "/v1/responses",
		ClientHeaders: map[string]string{"Authorization": "[REDACTED]"},
		ClientBody:    []byte(`{"model":"test"}`), UpstreamURL: "https://example.test/v1/responses",
		UpstreamBody: []byte(`{"model":"mapped"}`), ClientRespStatus: 502,
		ClientRespBody: []byte(`{"error":"failed"}`),
	}
	if err := saveFullLogPayloadSync(log.Id, &capture); err != nil {
		t.Fatal(err)
	}
	payload, err := GetLogPayload(log.Id)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"client request":   payload.ClientRequest,
		"upstream request": payload.UpstreamRequest,
		"client response":  payload.ClientResponse,
	} {
		if value == "" {
			t.Fatalf("%s was not serialized", name)
		}
	}
	var refreshed Log
	if err := DB.First(&refreshed, log.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !refreshed.HasFullRecord || refreshed.PayloadOriginalSize == 0 || refreshed.PayloadCompressedSize == 0 {
		t.Fatalf("payload metadata not updated: %+v", refreshed)
	}
}

func TestLogQueryFilters(t *testing.T) {
	setupTestDB(t)

	// 创建测试日志
	logs := []*Log{
		{RequestId: "req1", Type: LogTypeConsume, Status: 200, IsStream: false, HasFullRecord: true},
		{RequestId: "req2", Type: LogTypeConsume, Status: 500, IsStream: true, HasFullRecord: false},
		{RequestId: "req3", Type: LogTypeError, Status: 404, IsStream: false, HasFullRecord: true},
	}
	for _, l := range logs {
		if err := CreateLog(l); err != nil {
			t.Fatal(err)
		}
	}

	// 测试 has_full_record 筛选
	t.Run("has_full_record", func(t *testing.T) {
		hasRecord := true
		q := &LogQuery{HasFullRecord: &hasRecord, Page: 1, PageSize: 10}
		_, total, err := ListLogs(q)
		if err != nil {
			t.Fatal(err)
		}
		if total != 2 {
			t.Errorf("expected 2 logs with full record, got %d", total)
		}
	})

	// 测试 is_stream 筛选
	t.Run("is_stream", func(t *testing.T) {
		isStream := true
		q := &LogQuery{IsStream: &isStream, Page: 1, PageSize: 10}
		result, total, err := ListLogs(q)
		if err != nil {
			t.Fatal(err)
		}
		if total != 1 {
			t.Errorf("expected 1 stream log, got %d", total)
		}
		if len(result) > 0 && result[0].RequestId != "req2" {
			t.Errorf("expected req2, got %s", result[0].RequestId)
		}
	})

	// 测试 status 范围筛选
	t.Run("status_range", func(t *testing.T) {
		q := &LogQuery{StatusMin: 400, StatusMax: 599, Page: 1, PageSize: 10}
		_, total, err := ListLogs(q)
		if err != nil {
			t.Fatal(err)
		}
		if total < 2 {
			t.Errorf("expected at least 2 error status logs, got %d", total)
		}
	})
}
