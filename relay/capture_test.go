package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
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

// 采集缓冲必须有字节上限：流式响应逐块累积，无上限会随会话长度线性吃内存。
func TestBoundedBufferTruncatesBeyondLimit(t *testing.T) {
	b := newBoundedBuffer(10)
	n, err := b.Write([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// 必须返回完整写入长度，否则 TeeReader 会把短写当错误并中断上游读取。
	if n != 16 {
		t.Fatalf("Write returned %d, want 16 (short writes break io.TeeReader)", n)
	}
	got := string(b.Bytes())
	if !strings.HasPrefix(got, "0123456789") {
		t.Fatalf("prefix lost: %q", got)
	}
	if !strings.Contains(got, "truncated by apirelay") {
		t.Fatalf("missing truncation marker: %q", got)
	}
}

func TestBoundedBufferKeepsSmallPayloadIntact(t *testing.T) {
	b := newBoundedBuffer(1024)
	if _, err := b.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte(" world")); err != nil {
		t.Fatal(err)
	}
	if got := string(b.Bytes()); got != "hello world" {
		t.Fatalf("body = %q", got)
	}
}

func TestAppendCappedStopsAtLimit(t *testing.T) {
	out := appendCapped(nil, []byte("abcde"), 3)
	if !strings.HasPrefix(string(out), "abc") {
		t.Fatalf("prefix = %q", out)
	}
	if !strings.Contains(string(out), "truncated by apirelay") {
		t.Fatalf("missing marker: %q", out)
	}
	// 超限后继续追加不应无限增长，也不应重复叠加标记。
	before := len(out)
	out = appendCapped(out, []byte("fghij"), 3)
	if len(out) != before {
		t.Fatalf("length grew past limit: %d -> %d", before, len(out))
	}
}

func TestAppendCappedUnderLimitAppendsAll(t *testing.T) {
	out := appendCapped(nil, []byte("ab"), 10)
	out = appendCapped(out, []byte("cd"), 10)
	if string(out) != "abcd" {
		t.Fatalf("out = %q", out)
	}
}

// 流式采集在超限后必须停止累积，且不能影响转发本身。
func TestCaptureResponseWriterCapsBodyGrowth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	capture := &relaycommon.FullLogCapture{}
	cfg := &model.LoggingConfig{RecordClientResp: true}
	writer := &captureResponseWriter{ResponseWriter: ctx.Writer, capture: capture, cfg: cfg}
	ctx.Writer = writer

	chunk := bytes.Repeat([]byte("x"), 64*1024)
	written := 0
	for i := 0; i < 40; i++ { // 累计约 2.5MB，超过 1MB 采集上限
		n, err := ctx.Writer.Write(chunk)
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		written += n
	}

	// 转发本身不受采集上限影响：客户端仍应收到全部字节。
	if recorder.Body.Len() != written {
		t.Fatalf("client received %d bytes, wrote %d", recorder.Body.Len(), written)
	}
	if len(capture.ClientRespBody) > maxCapturedBodyBytes+len(capturedTruncationMarker) {
		t.Fatalf("capture grew to %d bytes, limit is %d", len(capture.ClientRespBody), maxCapturedBodyBytes)
	}
	if !bytes.Contains(capture.ClientRespBody, []byte("truncated by apirelay")) {
		t.Fatal("capture missing truncation marker")
	}
}
