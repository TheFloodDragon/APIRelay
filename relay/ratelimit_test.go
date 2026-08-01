package relay

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

var hintBase = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func headerWith(pairs ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(pairs); i += 2 {
		h.Set(pairs[i], pairs[i+1])
	}
	return h
}

func TestParseRetryAfterSeconds(t *testing.T) {
	hint := parseRateLimitHint(headerWith("Retry-After", "30"), hintBase)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter != 30*time.Second {
		t.Fatalf("retry after = %v, want 30s", hint.RetryAfter)
	}
	if hint.Source != "Retry-After" {
		t.Fatalf("source = %q", hint.Source)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	at := hintBase.Add(90 * time.Second)
	hint := parseRateLimitHint(headerWith("Retry-After", at.Format(http.TimeFormat)), hintBase)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter < 89*time.Second || hint.RetryAfter > 91*time.Second {
		t.Fatalf("retry after = %v, want ~90s", hint.RetryAfter)
	}
}

// 过去的时刻表示配额已恢复，不应产生冷却。
func TestParseRetryAfterIgnoresPastDate(t *testing.T) {
	at := hintBase.Add(-time.Minute)
	if hint := parseRateLimitHint(headerWith("Retry-After", at.Format(http.TimeFormat)), hintBase); hint != nil {
		t.Fatalf("past date should yield no hint, got %v", hint.RetryAfter)
	}
}

func TestParseRetryAfterIgnoresZeroAndNegative(t *testing.T) {
	for _, raw := range []string{"0", "-5"} {
		if hint := parseRateLimitHint(headerWith("Retry-After", raw), hintBase); hint != nil {
			t.Fatalf("Retry-After %q should yield no hint, got %v", raw, hint.RetryAfter)
		}
	}
}

// OpenAI 用 Go duration 字符串表达重置时间。
func TestParseOpenAIResetHeaders(t *testing.T) {
	cases := map[string]time.Duration{
		"6m0s": 6 * time.Minute,
		"1s":   time.Second,
		"20ms": 20 * time.Millisecond,
	}
	for raw, want := range cases {
		hint := parseRateLimitHint(headerWith("X-Ratelimit-Reset-Requests", raw), hintBase)
		if hint == nil {
			t.Fatalf("value %q: expected a hint", raw)
		}
		if hint.RetryAfter != want {
			t.Fatalf("value %q: retry after = %v, want %v", raw, hint.RetryAfter, want)
		}
	}
}

// Anthropic 用 RFC3339 绝对时刻。
func TestParseAnthropicResetHeader(t *testing.T) {
	at := hintBase.Add(2 * time.Minute)
	hint := parseRateLimitHint(
		headerWith("Anthropic-Ratelimit-Requests-Reset", at.Format(time.RFC3339)),
		hintBase,
	)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter != 2*time.Minute {
		t.Fatalf("retry after = %v, want 2m", hint.RetryAfter)
	}
}

// 通用 x-ratelimit-reset 可能是 Unix 秒、Unix 毫秒或相对秒数。
func TestParseGenericResetHeaderNumericForms(t *testing.T) {
	t.Run("unix 秒", func(t *testing.T) {
		at := hintBase.Add(45 * time.Second)
		hint := parseRateLimitHint(
			headerWith("X-Ratelimit-Reset", formatInt(at.Unix())),
			hintBase,
		)
		if hint == nil || hint.RetryAfter != 45*time.Second {
			t.Fatalf("hint = %+v, want 45s", hint)
		}
	})

	t.Run("unix 毫秒", func(t *testing.T) {
		at := hintBase.Add(45 * time.Second)
		hint := parseRateLimitHint(
			headerWith("X-Ratelimit-Reset", formatInt(at.UnixMilli())),
			hintBase,
		)
		if hint == nil || hint.RetryAfter != 45*time.Second {
			t.Fatalf("hint = %+v, want 45s", hint)
		}
	})

	t.Run("相对秒数", func(t *testing.T) {
		hint := parseRateLimitHint(headerWith("X-Ratelimit-Reset", "12"), hintBase)
		if hint == nil || hint.RetryAfter != 12*time.Second {
			t.Fatalf("hint = %+v, want 12s", hint)
		}
	})
}

// Retry-After 语义最明确，应优先于各厂商的 reset 头。
func TestRetryAfterTakesPrecedenceOverResetHeaders(t *testing.T) {
	hint := parseRateLimitHint(headerWith(
		"Retry-After", "10",
		"X-Ratelimit-Reset-Requests", "5m0s",
	), hintBase)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter != 10*time.Second || hint.Source != "Retry-After" {
		t.Fatalf("hint = %+v, want Retry-After 10s", hint)
	}
}

// 多个 reset 头同时存在时取最小值：任一维度恢复就值得重试。
func TestResetHeadersPickSmallestWait(t *testing.T) {
	hint := parseRateLimitHint(headerWith(
		"X-Ratelimit-Reset-Requests", "5m0s",
		"X-Ratelimit-Reset-Tokens", "20s",
	), hintBase)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter != 20*time.Second {
		t.Fatalf("retry after = %v, want the smallest (20s)", hint.RetryAfter)
	}
	if hint.Source != "X-Ratelimit-Reset-Tokens" {
		t.Fatalf("source = %q", hint.Source)
	}
}

// 超长重置时间（如按月配额）必须被截断，否则渠道会长期不可用。
func TestParseRateLimitHintClampsExcessiveWait(t *testing.T) {
	hint := parseRateLimitHint(headerWith("Retry-After", "86400"), hintBase)
	if hint == nil {
		t.Fatal("expected a hint")
	}
	if hint.RetryAfter != maxRateLimitCooldown {
		t.Fatalf("retry after = %v, want it clamped to %v", hint.RetryAfter, maxRateLimitCooldown)
	}
}

func TestParseRateLimitHintReturnsNilWithoutHeaders(t *testing.T) {
	if hint := parseRateLimitHint(http.Header{}, hintBase); hint != nil {
		t.Fatalf("expected nil, got %+v", hint)
	}
	if hint := parseRateLimitHint(nil, hintBase); hint != nil {
		t.Fatalf("expected nil for nil header, got %+v", hint)
	}
}

func TestParseRateLimitHintIgnoresUnparseableValues(t *testing.T) {
	hint := parseRateLimitHint(headerWith(
		"Retry-After", "soon",
		"X-Ratelimit-Reset", "not-a-number",
	), hintBase)
	if hint != nil {
		t.Fatalf("expected nil for garbage values, got %+v", hint)
	}
}

func TestIsRateLimitStatus(t *testing.T) {
	for _, status := range []int{429, 403, 503, 504} {
		if !isRateLimitStatus(status) {
			t.Errorf("status %d should be treated as rate limited", status)
		}
	}
	for _, status := range []int{200, 400, 401, 404, 500, 502} {
		if isRateLimitStatus(status) {
			t.Errorf("status %d should not be treated as rate limited", status)
		}
	}
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
