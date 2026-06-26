package adaptor

import (
	"strings"
	"testing"
)

func TestStreamRawLines_PreservesEventAndBlankLines(t *testing.T) {
	// 模拟 Anthropic SSE：强依赖 event: 行与空行边界
	input := "event: message_start\n" +
		"data: {\"type\":\"message_start\"}\n" +
		"\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n" +
		"\n" +
		": ping comment\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n" +
		"\n"

	var lines []string
	err := StreamRawLines(strings.NewReader(input), func(line string) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamRawLines error: %v", err)
	}

	want := []string{
		"event: message_start",
		"data: {\"type\":\"message_start\"}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}",
		"",
		": ping comment",
		"event: message_stop",
		"data: {\"type\":\"message_stop\"}",
		"",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %#v", len(lines), len(want), lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestParseSSEData(t *testing.T) {
	cases := []struct {
		line     string
		wantData string
		wantOK   bool
	}{
		{"data: {\"a\":1}", "{\"a\":1}", true},
		{"data:[DONE]", "[DONE]", true},
		{"event: foo", "", false},
		{"", "", false},
		{": comment", "", false},
	}
	for _, c := range cases {
		got, ok := ParseSSEData(c.line)
		if ok != c.wantOK || got != c.wantData {
			t.Errorf("ParseSSEData(%q) = (%q,%v), want (%q,%v)", c.line, got, ok, c.wantData, c.wantOK)
		}
	}
}

func TestScanSSE_SkipsEventLines(t *testing.T) {
	// ScanSSE 仅用于纯解析，应跳过 event/空行/注释，只回调 data
	input := "event: x\ndata: a\n\nevent: y\ndata: b\ndata: [DONE]\ndata: c\n"
	var got []string
	sc := newSSEScanner(strings.NewReader(input))
	err := ScanSSE(sc, func(data string) error {
		got = append(got, data)
		return nil
	})
	if err != nil {
		t.Fatalf("ScanSSE error: %v", err)
	}
	want := []string{"a", "b"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v (should stop at [DONE])", got, want)
	}
}
