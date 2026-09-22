package keypool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/apirelay/apirelay/model"
)

// Manager 管理所有 Key 的运行时状态机（per-key）。
//
// 设计对称参照 relay/circuitbreaker.Manager，但更轻量：Key 级切换成本低，
// 只保留顺序选择 + 冷却/失效所需的状态，不做错误率滑动窗口统计。
type Manager struct {
	cfg    Config
	cfgMu  sync.RWMutex
	states sync.Map // map[int]*keyState，仅对有正 Id 的 Key 建状态
	// rrCursor 预留：round_robin 策略下 per-channel 轮转游标。
	rrCursor sync.Map // map[int]*atomic.Uint64
}

// globalManager 用 atomic.Pointer 保存，避免裸指针 + sync.Once 的前置读竞争
// （与 circuitbreaker.globalManager 相同的并发安全惰性初始化模式）。
var globalManager atomic.Pointer[Manager]
var initOnce sync.Once

// InitManager 初始化全局 Key 轮询管理器（仅首次调用生效）。
func InitManager(cfg Config) {
	initOnce.Do(func() {
		globalManager.Store(&Manager{cfg: cfg.normalized()})
	})
}

// GetManager 获取全局管理器；未初始化时用默认配置惰性初始化。
func GetManager() *Manager {
	if m := globalManager.Load(); m != nil {
		return m
	}
	InitManager(DefaultConfig())
	return globalManager.Load()
}

// UpdateConfig 更新全局策略配置。
func (m *Manager) UpdateConfig(cfg Config) {
	cfg = cfg.normalized()
	m.cfgMu.Lock()
	m.cfg = cfg
	m.cfgMu.Unlock()
}

func (m *Manager) config() Config {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.cfg
}

// getState 获取（懒加载）指定 Key 的状态机，并从持久化记录恢复运行时字段。
// 虚拟 Key（Id<=0）不建状态，返回 nil。
func (m *Manager) getState(k *model.ChannelKey) *keyState {
	if k == nil || k.Id <= 0 {
		return nil
	}
	if v, ok := m.states.Load(k.Id); ok {
		return v.(*keyState)
	}
	// 从传入记录恢复初始状态（记录来自 DB，含重启前落库的运行时字段）。
	st := &keyState{
		keyID:               k.Id,
		channelID:           k.ChannelId,
		cooldownUntil:       k.CooldownUntil,
		consecutiveFailures: k.ConsecutiveFailures,
		totalRequests:       k.TotalRequests,
		failedRequests:      k.FailedRequests,
		disabled:            !k.Available,
		disabledReason:      model.DisableReason(k.DisabledReason),
		lastError:           k.LastError,
		lastFailureAtMs:     k.LastFailureAt,
		lastSuccessAtMs:     k.LastSuccessAt,
		persistVersion:      k.PersistVersion,
	}
	actual, _ := m.states.LoadOrStore(k.Id, st)
	return actual.(*keyState)
}

// SelectKey 从候选 Key 中选择一个用于本次尝试。
//
// keys 须已按 KeyIndex 升序（LoadEnabledChannelKeys 保证）。策略：
//   - sequential（本期）：顺序遍历，跳过 excluded、失效、冷却中的 Key，返回首个可用。
//   - round_robin（预留）：从 per-channel 游标位置开始环形扫描。
//
// 虚拟 Key（Id<=0）视为恒可用（除非被 excluded）：回退兼容层用它承载渠道级单 Key，
// 其健康交由渠道级熔断/冷却处理，keypool 不为其维护状态。
func (m *Manager) SelectKey(keys []*model.ChannelKey, excluded map[int]struct{}, nowMs int64) *model.ChannelKey {
	if len(keys) == 0 {
		return nil
	}
	if m.config().Strategy == StrategyRoundRobin {
		return m.selectRoundRobin(keys, excluded, nowMs)
	}
	return m.selectSequential(keys, excluded, nowMs)
}

func (m *Manager) selectSequential(keys []*model.ChannelKey, excluded map[int]struct{}, nowMs int64) *model.ChannelKey {
	for _, k := range keys {
		if m.candidateAvailable(k, excluded, nowMs) {
			return k
		}
	}
	return nil
}

// selectRoundRobin 从 per-channel 游标位置开始环形扫描第一个可用 Key。
// 每次成功选中后推进游标，使负载在可用 Key 间轮转。
func (m *Manager) selectRoundRobin(keys []*model.ChannelKey, excluded map[int]struct{}, nowMs int64) *model.ChannelKey {
	channelID := keys[0].ChannelId
	cursor := m.cursorFor(channelID)
	start := int(cursor.Load() % uint64(len(keys)))
	for i := 0; i < len(keys); i++ {
		idx := (start + i) % len(keys)
		if m.candidateAvailable(keys[idx], excluded, nowMs) {
			cursor.Store(uint64(idx + 1))
			return keys[idx]
		}
	}
	return nil
}

func (m *Manager) cursorFor(channelID int) *atomic.Uint64 {
	if v, ok := m.rrCursor.Load(channelID); ok {
		return v.(*atomic.Uint64)
	}
	c := &atomic.Uint64{}
	actual, _ := m.rrCursor.LoadOrStore(channelID, c)
	return actual.(*atomic.Uint64)
}

// candidateAvailable 判断某 Key 是否可作为本次尝试的候选。
func (m *Manager) candidateAvailable(k *model.ChannelKey, excluded map[int]struct{}, nowMs int64) bool {
	if k == nil {
		return false
	}
	if excluded != nil {
		if _, skip := excluded[k.Id]; skip {
			return false
		}
	}
	// 虚拟 Key：无独立状态，恒可用（健康交给渠道级）。
	if k.Id <= 0 {
		return true
	}
	st := m.getState(k)
	if st == nil {
		return true
	}
	return st.availableAt(nowMs)
}

// RecordSuccess 记录一次成功：清零失败计数、解除冷却与失效，并落库。
func (m *Manager) RecordSuccess(k *model.ChannelKey) {
	st := m.getState(k)
	if st == nil {
		return
	}
	st.mu.Lock()
	st.recordSuccessLocked(time.Now().UnixMilli())
	health := st.snapshotLocked()
	st.mu.Unlock()
	queuePersist(health)
}

// RecordFailure 记录一次失败并按原因决定冷却或失效。
//
//   - auth_failed / quota_exhausted：按配置立即硬失效（AuthFailDisable=true），否则冷却。
//   - rate_limited：冷却 retryAfter（>0 时）或配置冷却时长。
//   - upstream_error（含 timeout）：冷却配置时长，连续失败累计达阈值则硬失效。
//
// 返回本次是否已将该 Key 置为硬失效（供调用方决定是否触发主动探测恢复路径）。
func (m *Manager) RecordFailure(k *model.ChannelKey, reason model.DisableReason, retryAfter time.Duration, errMsg string) bool {
	st := m.getState(k)
	if st == nil {
		return false // 虚拟 Key：健康交给渠道级
	}
	cfg := m.config()
	nowMs := time.Now().UnixMilli()

	st.mu.Lock()
	st.totalRequests++
	st.failedRequests++
	st.consecutiveFailures++
	st.lastError = truncate(errMsg, 500)
	st.lastFailureAtMs = nowMs

	switch reason {
	case model.DisableReasonAuthFailed, model.DisableReasonQuotaExhausted:
		if cfg.AuthFailDisable {
			st.disabled = true
			st.disabledReason = reason
		} else {
			st.cooldownUntil = nowMs + cooldownDuration(cfg.CooldownSeconds).Milliseconds()
		}
	case model.DisableReasonRateLimited:
		wait := retryAfter
		if wait <= 0 {
			wait = cooldownDuration(cfg.CooldownSeconds)
		}
		st.cooldownUntil = nowMs + wait.Milliseconds()
	default: // upstream_error / timeout 等
		st.cooldownUntil = nowMs + cooldownDuration(cfg.CooldownSeconds).Milliseconds()
		if st.consecutiveFailures >= cfg.FailureThreshold {
			st.disabled = true
			st.disabledReason = model.DisableReasonUpstreamError
		}
	}

	disabled := st.disabled
	health := st.snapshotLocked()
	st.mu.Unlock()

	queuePersist(health)
	return disabled
}

// MarkDisabled 手动/外部将 Key 置为硬失效并落库。
func (m *Manager) MarkDisabled(k *model.ChannelKey, reason model.DisableReason) {
	st := m.getState(k)
	if st == nil {
		return
	}
	st.mu.Lock()
	st.disabled = true
	st.disabledReason = reason
	st.lastFailureAtMs = time.Now().UnixMilli()
	health := st.snapshotLocked()
	st.mu.Unlock()
	queuePersist(health)
}

// MarkRecovered 将 Key 恢复为可用（清冷却/失效/失败计数）并落库。
// 供主动探测 worker 在探测成功后调用。
func (m *Manager) MarkRecovered(k *model.ChannelKey) {
	st := m.getState(k)
	if st == nil {
		return
	}
	st.mu.Lock()
	st.disabled = false
	st.disabledReason = model.DisableReasonNone
	st.cooldownUntil = 0
	st.consecutiveFailures = 0
	st.lastError = ""
	st.lastSuccessAtMs = time.Now().UnixMilli()
	health := st.snapshotLocked()
	st.mu.Unlock()
	// 恢复是低频、非热路径且对持久化敏感的状态跃迁（内存已恢复，但需落库保证重启后不回退失效）。
	// 异步队列是为热路径高失败率下防止 goroutine 爆炸而设，这里不适用，故同步落库。
	// persistVersion 单调递增 + UpsertChannelKeyHealth 的版本守卫保证在途旧快照不会覆盖本次恢复。
	persistSnapshot(health)
}

// Snapshot 返回某 Key 的只读运行时快照；无状态（虚拟 Key/未加载）返回零值与 false。
func (m *Manager) Snapshot(keyID int) (KeyRuntime, bool) {
	if keyID <= 0 {
		return KeyRuntime{}, false
	}
	v, ok := m.states.Load(keyID)
	if !ok {
		return KeyRuntime{}, false
	}
	st := v.(*keyState)
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.runtimeLocked(), true
}

// Forget 丢弃某 Key 的内存状态（删除 Key 后调用，避免 sync.Map 泄漏）。
func (m *Manager) Forget(keyID int) {
	if keyID > 0 {
		m.states.Delete(keyID)
	}
}

// truncate 截断错误信息，避免无界文本占用内存与落库。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
