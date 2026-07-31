package controller

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
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

func TestLogQueryRejectsInvalidBooleanAndRange(t *testing.T) {
	router := setupLogControllerTest(t)
	for _, path := range []string{
		"/api/logs?is_stream=maybe",
		"/api/logs?status_min=500&status_max=400",
		"/api/logs/export?start_time=20&end_time=10",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body = %s", path, recorder.Code, recorder.Body.String())
		}
	}
}
