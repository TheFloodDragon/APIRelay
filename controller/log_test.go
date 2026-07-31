package controller

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
	"github.com/gin-gonic/gin"
)

func setupLogControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file:controller-log-export?mode=memory&cache=shared"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDatabases() })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/logs", ListLogs)
	router.GET("/api/logs/export", ExportLogs)
	router.GET("/api/logs/:id", GetLogDetail)
	return router
}

func TestExportLogsCSVUsesFiltersAndPreservesText(t *testing.T) {
	router := setupLogControllerTest(t)
	createdAt := int64(1_700_000_000_123)
	matching := &model.Log{
		RequestId:   "req-export",
		CreatedAt:   createdAt,
		Type:        model.LogTypeError,
		TokenName:   "token-a",
		SrcModel:    "gpt-4o",
		Status:      502,
		Error:       "上游失败, retry",
		Content:     "line one\nline \"two\"",
		IsStream:    true,
		TotalTokens: 12,
	}
	if err := model.CreateLog(matching); err != nil {
		t.Fatal(err)
	}
	if err := model.CreateLog(&model.Log{RequestId: "req-other", CreatedAt: createdAt + 1, Type: model.LogTypeConsume, SrcModel: "claude", Status: 200}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/logs/export?model=gpt-4o&page=1&page_size=1", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := recorder.Header().Get("X-Export-Count"); got != "1" {
		t.Fatalf("export count = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}

	raw := recorder.Body.Bytes()
	if len(raw) < 3 || !bytes.Equal(raw[:3], []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("missing UTF-8 BOM: %v", raw[:min(len(raw), 3)])
	}
	rows, err := csv.NewReader(bytes.NewReader(raw[3:])).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv rows = %d, want header + 1", len(rows))
	}
	if len(rows[0]) != 34 || rows[0][33] != "created_at_utc" {
		t.Fatalf("csv header = %#v", rows[0])
	}
	indexes := make(map[string]int, len(rows[0]))
	for index, name := range rows[0] {
		indexes[name] = index
	}
	data := rows[1]
	if data[indexes["request_id"]] != "req-export" || data[indexes["error"]] != matching.Error || data[indexes["content"]] != matching.Content {
		t.Fatalf("csv data = %#v", data)
	}
	if data[indexes["created_at_utc"]] != "2023-11-14T22:13:20.123Z" {
		t.Fatalf("created_at_utc = %q", data[indexes["created_at_utc"]])
	}
	if _, exists := indexes["client_request"]; exists {
		t.Fatal("CSV must not expand log payload fields")
	}
}

func TestExportLogsIncludesPayloadColumnsOnlyWhenRequested(t *testing.T) {
	router := setupLogControllerTest(t)
	createdAt := int64(1_700_000_000_123)
	full := &model.Log{RequestId: "req-full", CreatedAt: createdAt, Type: model.LogTypeConsume, SrcModel: "gpt-4o", Status: 200}
	if err := model.CreateLog(full); err != nil {
		t.Fatal(err)
	}
	if err := model.CreateLogPayload(full.Id, &model.FullLogData{
		ClientRequest:    `{"model":"gpt-4o","messages":[{"role":"user","content":"你好, \"世界\""}]}`,
		ClientResponse:   "data: chunk-1\ndata: chunk-2\n",
		FailoverAttempts: `[{"decision":"success"}]`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := model.CreateLog(&model.Log{RequestId: "req-summary-only", CreatedAt: createdAt + 1, Type: model.LogTypeConsume, SrcModel: "gpt-4o", Status: 200}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/logs/export?include_payload=true", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "apirelay-logs-full-") {
		t.Fatalf("content disposition = %q", got)
	}

	rows, err := csv.NewReader(bytes.NewReader(recorder.Body.Bytes()[3:])).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("csv rows = %d, want header + 2", len(rows))
	}
	indexes := make(map[string]int, len(rows[0]))
	for index, name := range rows[0] {
		indexes[name] = index
	}
	for _, name := range []string{"failover_attempts", "client_request", "upstream_request", "upstream_response", "client_response"} {
		if _, exists := indexes[name]; !exists {
			t.Fatalf("missing payload column %q in %#v", name, rows[0])
		}
	}
	if len(rows[0]) != len(rows[1]) || len(rows[0]) != len(rows[2]) {
		t.Fatalf("row widths = %d/%d/%d", len(rows[0]), len(rows[1]), len(rows[2]))
	}

	byRequest := map[string][]string{rows[1][indexes["request_id"]]: rows[1], rows[2][indexes["request_id"]]: rows[2]}
	fullRow := byRequest["req-full"]
	if fullRow == nil {
		t.Fatalf("missing req-full row: %#v", rows)
	}
	if fullRow[indexes["client_request"]] != `{"model":"gpt-4o","messages":[{"role":"user","content":"你好, \"世界\""}]}` {
		t.Fatalf("client_request = %q", fullRow[indexes["client_request"]])
	}
	if fullRow[indexes["client_response"]] != "data: chunk-1\ndata: chunk-2\n" {
		t.Fatalf("client_response = %q", fullRow[indexes["client_response"]])
	}
	if fullRow[indexes["failover_attempts"]] != `[{"decision":"success"}]` {
		t.Fatalf("failover_attempts = %q", fullRow[indexes["failover_attempts"]])
	}
	summaryRow := byRequest["req-summary-only"]
	if summaryRow == nil {
		t.Fatalf("missing req-summary-only row: %#v", rows)
	}
	for _, name := range []string{"client_request", "client_response", "failover_attempts"} {
		if summaryRow[indexes[name]] != "" {
			t.Fatalf("log without payload should keep %s empty, got %q", name, summaryRow[indexes[name]])
		}
	}
}

func TestLogQueryRejectsInvalidBooleanAndRange(t *testing.T) {
	router := setupLogControllerTest(t)
	for _, path := range []string{
		"/api/logs?is_stream=maybe",
		"/api/logs?status_min=500&status_max=400",
		"/api/logs/export?start_time=20&end_time=10",
		"/api/logs/export?include_payload=maybe",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, recorder.Code, recorder.Body.String())
		}
	}
}
