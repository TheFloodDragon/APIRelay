package anthropic

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// fakeBody 将字符串包装为 ReadCloser。
func fakeBody(s string) io.ReadCloser { return io.NopCloser(strings.NewReader(s)) }

const anthropicStream = "event: message_start\n" +
	"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n" +
	"\n" +
	"event: content_block_delta\n" +
	"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n" +
	"\n" +
	"event: message_delta\n" +
	"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n" +
	"\n" +
	"event: message_stop\n" +
	"data: {\"type\":\"message_stop\"}\n" +
	"\n"

// 同协议透传：event: 行与空行必须完整保留。
func TestStreamHandler_AnthropicPassthroughKeepsEventLines(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{EndpointType: constant.EndpointAnthropic}
	resp := &http.Response{Body: fakeBody(anthropicStream)}

	var sb strings.Builder
	usage, err := a.StreamHandler(info, resp, func(chunk *dto.UnifiedStreamChunk) error {
		if !chunk.IsRaw {
			t.Fatalf("expected IsRaw chunk in passthrough mode, got %#v", chunk)
		}
		sb.WriteString(chunk.Raw)
		sb.WriteString("\n")
		return nil
	})
	if err != nil {
		t.Fatalf("StreamHandler error: %v", err)
	}

	out := sb.String()
	for _, want := range []string{
		"event: message_start",
		"event: content_block_delta",
		"event: message_delta",
		"event: message_stop",
		"data: {\"type\":\"content_block_delta\"",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}

	// usage 旁路解析应提取到 output_tokens
	if usage == nil {
		t.Fatal("expected usage extracted via bypass parse")
	}
	if usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d, want 5", usage.CompletionTokens)
	}
}

// 跨协议（对外 OpenAI）：应走 IR 解析，产出 DeltaText。
func TestStreamHandler_CrossProtocolParsesIR(t *testing.T) {
	a := &Adaptor{}
	info := &relaycommon.RelayInfo{EndpointType: constant.EndpointOpenAI}
	resp := &http.Response{Body: fakeBody(anthropicStream)}

	var text strings.Builder
	sawRaw := false
	_, err := a.StreamHandler(info, resp, func(chunk *dto.UnifiedStreamChunk) error {
		if chunk.IsRaw {
			sawRaw = true
		}
		text.WriteString(chunk.DeltaText)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamHandler error: %v", err)
	}
	if sawRaw {
		t.Error("cross-protocol mode should not emit raw chunks")
	}
	if text.String() != "Hello" {
		t.Errorf("DeltaText = %q, want %q", text.String(), "Hello")
	}
}
