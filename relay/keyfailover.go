package relay

import (
	"net/http"
	"strings"
	"time"

	"github.com/apirelay/apirelay/model"
)

// KeyDecision 表示一次 Key 级失败后的处置决策。
type KeyDecision int

const (
	// KeyDecisionRetrySameKey 在同一个 Key 上重试（瞬时错误，如 429/5xx，预算内）。
	KeyDecisionRetrySameKey KeyDecision = iota
	// KeyDecisionSwitchKey 冷却/失效当前 Key 并切换到下一个可用 Key（渠道内故障转移）。
	KeyDecisionSwitchKey
	// KeyDecisionSwitchChannel 跳出渠道内 Key 循环，切换到其它渠道（渠道级问题，如连接失败）。
	KeyDecisionSwitchChannel
	// KeyDecisionFatal 不可重试，直接返回错误（如响应已写出、客户端错误）。
	KeyDecisionFatal
)

func keyDecisionLabel(d KeyDecision) string {
	switch d {
	case KeyDecisionRetrySameKey:
		return "retry_same_key"
	case KeyDecisionSwitchKey:
		return "switch_key"
	case KeyDecisionSwitchChannel:
		return "switch_channel"
	case KeyDecisionFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// keyFailureOutcome 描述一次 Key 级失败的完整处置结果。
type keyFailureOutcome struct {
	Decision KeyDecision
	// Reason 传递给 keypool.RecordFailure 的失效原因（决定冷却 vs 硬失效）。
	Reason model.DisableReason
	// RecordKeyFailure 是否要把本次失败计入该 Key 的健康状态（冷却/失效）。
	// 同 Key 重试、响应已写出、客户端错误、渠道级传输错误等情形不计入，避免误伤 Key。
	RecordKeyFailure bool
}

// quotaMarkers 是上游"额度耗尽/欠费"错误文本的常见特征。
//
// 部分上游（如 OpenAI）对额度耗尽同样返回 429，若按普通限流冷却后重试，
// 只会反复失败；识别出配额语义后应直接失效该 Key，切换到有额度的 Key。
var quotaMarkers = []string{
	"insufficient_quota",
	"exceeded your current quota",
	"insufficient balance",
	"insufficient_user_quota",
	"quota exceeded",
	"out of credit",
	"billing",
	"欠费",
	"额度不足",
	"余额不足",
}

func hasQuotaSignal(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	for _, m := range quotaMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// classifyKeyFailure 将一次上游失败映射为 Key 级决策。
//
// 输入：
//   - status：上游 HTTP 状态码（或内部映射的状态）。
//   - category：relay 层错误分类（区分客户端取消 / relay 超时 / 上游超时 / 上游错误）。
//   - written：HTTP 响应体是否已开始写出（写出后任何错误都不可再重试，避免拼接双响应）。
//   - errMsg：干净化前的错误文本，用于识别配额语义与传输层错误。
//   - sameKeyRetries / keyMaxRetries：该 Key 已用 / 允许的同 Key 瞬时重试次数。
//
// 决策表（见设计文档第七节）：
//
//	client_canceled / relay_timeout        -> Fatal，不计 Key 失败（不误伤）
//	响应已写出                              -> Fatal，不计 Key 失败
//	传输层错误（连不上 base_url）            -> SwitchChannel（渠道级问题，换 Key 无益）
//	401                                    -> SwitchKey，auth_failed
//	402 / 含配额语义的 4xx                   -> SwitchKey，quota_exhausted
//	403                                    -> SwitchKey，auth_failed（或配额语义 -> quota_exhausted）
//	429（配额语义）                          -> SwitchKey，quota_exhausted（失效，不重试）
//	429（普通限流）                          -> 预算内 RetrySameKey，否则 SwitchKey，rate_limited（冷却）
//	5xx / 上游超时                          -> 预算内 RetrySameKey，否则 SwitchKey，upstream_error
//	其它不可重试                            -> Fatal
func classifyKeyFailure(status int, category RelayErrorCategory, written bool, errMsg string, sameKeyRetries, keyMaxRetries int) keyFailureOutcome {
	// 客户端主动取消 / relay 总超时：不是 Key 的问题，终止且不计失败。
	if category == ErrorCategoryClientCanceled || category == ErrorCategoryRelayTimeout {
		return keyFailureOutcome{Decision: KeyDecisionFatal, RecordKeyFailure: false}
	}

	// 响应已写出：不可再重试（否则把第二个上游响应拼接进已提交的流）。不计 Key 失败（可能只是流中断）。
	if written {
		return keyFailureOutcome{Decision: KeyDecisionFatal, RecordKeyFailure: false}
	}

	// 传输层错误（连接被拒 / DNS / base_url 不可达）：渠道级问题，换 Key 无益，直接切渠道。
	if category == ErrorCategoryUpstream && strings.HasPrefix(strings.TrimSpace(errMsg), "do request:") {
		return keyFailureOutcome{Decision: KeyDecisionSwitchChannel, Reason: model.DisableReasonNone, RecordKeyFailure: false}
	}

	quota := hasQuotaSignal(errMsg)

	switch {
	case status == http.StatusUnauthorized:
		// 401 鉴权失败：该 Key 的凭据失效，切下一个 Key。
		return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: model.DisableReasonAuthFailed, RecordKeyFailure: true}

	case status == http.StatusPaymentRequired:
		// 402 额度耗尽。
		return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: model.DisableReasonQuotaExhausted, RecordKeyFailure: true}

	case status == http.StatusForbidden:
		// 403：部分上游用它表达配额耗尽，另一些表达权限不足。二者都应切 Key。
		reason := model.DisableReasonAuthFailed
		if quota {
			reason = model.DisableReasonQuotaExhausted
		}
		return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: reason, RecordKeyFailure: true}

	case status == http.StatusTooManyRequests:
		// 429：先判配额语义（OpenAI 用 429 表达额度耗尽）——此时应失效而非重试。
		if quota {
			return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: model.DisableReasonQuotaExhausted, RecordKeyFailure: true}
		}
		// 普通限流：预算内先同 Key 重试（配合退避），否则冷却并切 Key。
		if sameKeyRetries < keyMaxRetries {
			return keyFailureOutcome{Decision: KeyDecisionRetrySameKey, Reason: model.DisableReasonRateLimited, RecordKeyFailure: false}
		}
		return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: model.DisableReasonRateLimited, RecordKeyFailure: true}

	case isTimeoutCategory(category) || (status >= 500 && status <= 599):
		// 上游 5xx / 超时：预算内同 Key 重试，否则冷却并切 Key；连续失败累计由 keypool 决定是否硬失效。
		if sameKeyRetries < keyMaxRetries {
			return keyFailureOutcome{Decision: KeyDecisionRetrySameKey, Reason: model.DisableReasonUpstreamError, RecordKeyFailure: false}
		}
		return keyFailureOutcome{Decision: KeyDecisionSwitchKey, Reason: model.DisableReasonUpstreamError, RecordKeyFailure: true}

	default:
		// 其它状态（如 400 参数错误）：不可重试，直接返回；换 Key 也无济于事。
		return keyFailureOutcome{Decision: KeyDecisionFatal, Reason: model.DisableReasonUpstreamError, RecordKeyFailure: false}
	}
}

// keyRetryDelay 在同 Key 重试前的退避时长（复用渠道级同渠道重试延迟节奏）。
const keyRetryDelay = 400 * time.Millisecond
