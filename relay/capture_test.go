package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/relaycommon"
	"github.com/gin-gonic/gin"
)

func TestSanitizeHeadersRedactsCaseInsensitively(t *testing.T) {
	headers := http.Header{
		"Authorization": []string{"Bearer secret"},
		"X-Api-Key":     []string{"key-secret"},
		"X-Trace":       []string{"trace-1", "trace-2"},
	}
	got := sanitizeHeaders(headers, []string{"authorization", "x-api-key"})
	if got["Authorization"] != "[REDACTED]" || got["X-Api-Key"] != "[REDACTED]" {
		t.Fatalf("sensitive headers were not redacted: %#v", got)
	}
	if got["X-Trace"] != "trace-1, trace-2" {
		t.Fatalf("non-sensitive header changed: %#v", got)
	}
}

func TestCaptureResponseWriterRecordsBodyStatusAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	capture := &relaycommon.FullLogCapture{}
	cfg := &model.LoggingConfig{RecordClientResp: true, SanitizedHeaderKeys: []string{"Set-Cookie"}}
	writer := &captureResponseWriter{ResponseWriter: ctx.Writer, capture: capture, cfg: cfg}
	ctx.Writer = writer

	ctx.Header("Set-Cookie", "session=secret")
	ctx.Header("X-Trace", "trace-1")
	ctx.Status(http.StatusAccepted)
	if _, err := ctx.Writer.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}

	if capture.ClientRespStatus != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", capture.ClientRespStatus, http.StatusAccepted)
	}
	if string(capture.ClientRespBody) != "payload" {
		t.Fatalf("body = %q", capture.ClientRespBody)
	}
	if capture.ClientRespHeaders["Set-Cookie"] != "[REDACTED]" {
		t.Fatalf("cookie not redacted: %#v", capture.ClientRespHeaders)
	}
	if capture.ClientRespHeaders["X-Trace"] != "trace-1" {
		t.Fatalf("trace header missing: %#v", capture.ClientRespHeaders)
	}
}
