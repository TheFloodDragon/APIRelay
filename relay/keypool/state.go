package keypool

import (
	"sync"
	"time"

	"github.com/apirelay/apirelay/model"
)

// keyState 是单个 Key 的运行时健康状态机（内存权威）。
//
// 每个 Key 独立维护可用状态、连续失败次数、冷却截止时间。状态翻转由 per-key
// mutex 保护；SelectKey 的读取走只读快照，落库经有界队列异步进行。
type keyState struct {
	mu        sync.Mutex
	keyID     int
	channelID int

	cooldownUntil       int64 // 毫秒时间戳，0 表示未冷却
	consecutiveFailures int
	totalRequests       int
	failedRequests      int
	disabled            bool // true=硬失效，需恢复检测后才回归
	disabledReason      model.DisableReason
	lastError           string
	lastFailureAtMs     int64
	lastSuccessAtMs     int64
	persistVersion      uint64
}

// KeyRuntime 是对外暴露的只读运行时快照（供管理 API / 探测 worker 观测）。
type KeyRuntime struct {
	KeyID               int                 `json:"key_id"`
	ChannelID           int                 `json:"channel_id"`
	Available           bool                `json:"available"`
	CooldownUntilMs     int64               `json:"cooldown_until"`
	ConsecutiveFailures int                 `json:"consecutive_failures"`
	TotalRequests       int                 `json:"total_requests"`
	FailedRequests      int                 `json:"failed_requests"`
	DisabledReason      model.DisableReason `json:"disabled_reason"`
	LastError           string              `json:"last_error"`
}

// availableAt 判断在给定时刻该 Key 是否可用（未失效且未处于冷却）。
func (s *keyState) availableAt(nowMs int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disabled {
		return false
	}
	return s.cooldownUntil <= nowMs
}

// snapshotLocked 生成落库快照并推进版本号（调用方须持有锁）。
//
// 版本号必须每次递增：UpsertChannelKeyHealth 依赖它丢弃过期快照，
// 否则并发落库的先后顺序随机，新状态可能被旧快照覆盖。
func (s *keyState) snapshotLocked() *model.ChannelKey {
	s.persistVersion++
	return &model.ChannelKey{
		Id:                  s.keyID,
		Available:           !s.disabled,
		CooldownUntil:       s.cooldownUntil,
		ConsecutiveFailures: s.consecutiveFailures,
		TotalRequests:       s.totalRequests,
		FailedRequests:      s.failedRequests,
		DisabledReason:      string(s.disabledReason),
		LastError:           s.lastError,
		LastFailureAt:       s.lastFailureAtMs,
		LastSuccessAt:       s.lastSuccessAtMs,
		PersistVersion:      s.persistVersion,
	}
}

// runtimeLocked 生成只读快照（调用方须持有锁）。
func (s *keyState) runtimeLocked() KeyRuntime {
	return KeyRuntime{
		KeyID:               s.keyID,
		ChannelID:           s.channelID,
		Available:           !s.disabled,
		CooldownUntilMs:     s.cooldownUntil,
		ConsecutiveFailures: s.consecutiveFailures,
		TotalRequests:       s.totalRequests,
		FailedRequests:      s.failedRequests,
		DisabledReason:      s.disabledReason,
		LastError:           s.lastError,
	}
}

// recordSuccessLocked 记录一次成功（调用方须持有锁）。
func (s *keyState) recordSuccessLocked(nowMs int64) {
	s.totalRequests++
	s.consecutiveFailures = 0
	s.cooldownUntil = 0
	s.disabled = false
	s.disabledReason = model.DisableReasonNone
	s.lastError = ""
	s.lastSuccessAtMs = nowMs
}

// cooldownDuration 将秒配置转换为 Duration。
func cooldownDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
