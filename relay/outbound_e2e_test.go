package relay

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/gin-gonic/gin"
)

// TestAnthropicRawPassthroughEndToEnd 验证：当对外协议为 Anthropic 时，
// 上游的 event: 行、空行边界经由 outbound writer 后被完整原样写出，
// 且 Finish 不会补发重复的合成终止事件。
func TestAnthropicRawPassthroughEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	out := GetOutbound(constant.EndpointAnthropic)
	w := out.NewStream(c, "req-1", "claude-3")

	// 模拟 StreamHandler 在 raw 模式下逐行回调
	rawLines := []string{
		"event: message_start",
		"data: {\"type\":\"message_start\"}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}",
		"",
		"event: message_stop",
		"data: {\"type\":\"message_stop\"}",
		"",
	}
	for _, line := range rawLines {
		if err := w.WriteChunk(c, &dto.UnifiedStreamChunk{Raw: line, IsRaw: true}); err != nil {
			t.Fatalf("WriteChunk: %v", err)
		}
	}
	if err := w.Finish(c); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	body := rec.Body.String()

	// event: 行必须存在
	for _, want := range []string{
		"event: message_start",
		"event: content_block_delta",
		"event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q\n---\n%s", want, body)
		}
	}

	// Finish 不应在 raw 模式补发额外的合成 message_start（避免重复终止/起始）
	if strings.Count(body, "event: message_stop") != 1 {
		t.Errorf("message_stop should appear exactly once, got:\n%s", body)
	}

	// 验证空行边界存在（事件之间应有空行）
	if !strings.Contains(body, "\n\n") {
		t.Errorf("output should preserve blank-line event boundaries:\n%s", body)
	}
}
