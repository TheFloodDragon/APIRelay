package relay

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 上游限流信息解析。
//
// 固定冷却时长有两个方向的错：冷却太短会在上游仍在限流时反复重试，
// 部分厂商会因此延长封禁；冷却太长则在配额早已恢复后仍把渠道排除在外，
// 白白浪费可用容量。
//
// 主流上游都会在 429/503 响应里明确告知恢复时刻，直接采用即可。
// 解析不出时回退到配置的固定冷却，因此这条路径是纯增强、不会让行为变差。

// rateLimitHint 描述从上游响应中解析出的限流恢复信息。
type rateLimitHint struct {
	// RetryAfter 距离可以重试还需等待多久。
	RetryAfter time.Duration
	// Source 记录命中的响应头名，用于日志排查。
	Source string
}

// maxRateLimitCooldown 限制解析结果的上界。
// 上游偶尔会返回极长的重置时间（如按月配额），若照搬会让渠道长期不可用；
// 到达上界后仍会在下一次请求时重新探测。
const maxRateLimitCooldown = 30 * time.Minute

// parseRateLimitHint 从上游响应头解析恢复时刻。
//
// 优先级：Retry-After（标准且语义明确）> 各厂商的 reset 头。
// 返回 nil 表示无法解析，调用方应回退固定冷却。
func parseRateLimitHint(header http.Header, now time.Time) *rateLimitHint {
	if header == nil {
		return nil
	}
	if hint := parseRetryAfter(header, now); hint != nil {
		return hint
	}
	return parseResetHeaders(header, now)
}

// parseRetryAfter 解析 RFC 7231 的 Retry-After，同时支持秒数与 HTTP-date 两种形式。
func parseRetryAfter(header http.Header, now time.Time) *rateLimitHint {
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if raw == "" {
		return nil
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil {
		if d := clampCooldown(time.Duration(seconds * float64(time.Second))); d > 0 {
			return &rateLimitHint{RetryAfter: d, Source: "Retry-After"}
		}
		return nil
	}
	if at, err := http.ParseTime(raw); err == nil {
		if d := clampCooldown(at.Sub(now)); d > 0 {
			return &rateLimitHint{RetryAfter: d, Source: "Retry-After"}
		}
	}
	return nil
}

// resetHeaderCandidates 是各厂商用于告知配额重置时刻的响应头。
//
// 取值形态不统一，需要逐个试探：
//   - OpenAI：x-ratelimit-reset-requests / -tokens，形如 "6m0s"、"1s"、"20ms"
//   - Anthropic：anthropic-ratelimit-*-reset，RFC3339 绝对时刻
//   - 通用：x-ratelimit-reset，可能是 Unix 秒、Unix 毫秒或相对秒数
var resetHeaderCandidates = []string{
	"X-Ratelimit-Reset-Requests",
	"X-Ratelimit-Reset-Tokens",
	"Anthropic-Ratelimit-Requests-Reset",
	"Anthropic-Ratelimit-Tokens-Reset",
	"Anthropic-Ratelimit-Input-Tokens-Reset",
	"Anthropic-Ratelimit-Output-Tokens-Reset",
	"X-Ratelimit-Reset",
	"Ratelimit-Reset",
}

// parseResetHeaders 遍历候选头，取其中最小的正等待时长。
//
// 取最小值而非最大值：只要任一维度的配额恢复，就值得重新尝试；
// 等到所有维度都恢复会过度保守。
func parseResetHeaders(header http.Header, now time.Time) *rateLimitHint {
	var best *rateLimitHint
	for _, name := range resetHeaderCandidates {
		raw := strings.TrimSpace(header.Get(name))
		if raw == "" {
			continue
		}
		d, ok := parseResetValue(raw, now)
		if !ok {
			continue
		}
		d = clampCooldown(d)
		if d <= 0 {
			continue
		}
		if best == nil || d < best.RetryAfter {
			best = &rateLimitHint{RetryAfter: d, Source: name}
		}
	}
	return best
}

// unixSecondsThreshold 用于区分「Unix 时间戳」与「相对秒数」。
// 约等于 2001-09-09，任何合理的相对等待都远小于它。
const unixSecondsThreshold = 1_000_000_000

// unixMillisThreshold 用于区分 Unix 秒与 Unix 毫秒。
const unixMillisThreshold = 100_000_000_000

// parseResetValue 解析单个 reset 头的值，返回距今的等待时长。
func parseResetValue(raw string, now time.Time) (time.Duration, bool) {
	// 形态 1：Go duration 字符串，OpenAI 使用（"6m0s"、"20ms"）。
	if d, err := time.ParseDuration(raw); err == nil {
		return d, true
	}
	// 形态 2：RFC3339 绝对时刻，Anthropic 使用。
	if at, err := time.Parse(time.RFC3339, raw); err == nil {
		return at.Sub(now), true
	}
	// 形态 3：数字。需要区分 Unix 毫秒 / Unix 秒 / 相对秒。
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		switch {
		case value >= unixMillisThreshold:
			return time.UnixMilli(int64(value)).Sub(now), true
		case value >= unixSecondsThreshold:
			return time.Unix(int64(value), 0).Sub(now), true
		default:
			return time.Duration(value * float64(time.Second)), true
		}
	}
	return 0, false
}

// clampCooldown 把等待时长限制到 [0, maxRateLimitCooldown]。
func clampCooldown(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d > maxRateLimitCooldown {
		return maxRateLimitCooldown
	}
	return d
}
