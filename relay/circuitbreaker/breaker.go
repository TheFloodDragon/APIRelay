package circuitbreaker

import (
	"sync"
	"time"

	"github.com/apirelay/apirelay/model"
)

// CircuitBreaker 熔断器实例（per-channel）
type CircuitBreaker struct {
	channelID            int
	cfg                  Config
	mu                   sync.RWMutex
	state                model.CircuitState
	openedAt             *time.Time
	consecutiveFailures  int
	consecutiveSuccesses int
	totalRequests        int
	failedRequests       int
	requestEvents        []requestEvent
	halfOpenInFlight     int
	persistVersion       uint64
	now                  func() time.Time
}

type requestEvent struct {
	at     time.Time
	failed bool
}

// NewCircuitBreaker 创建新的熔断器实例
func NewCircuitBreaker(channelID int, cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		channelID: channelID,
		cfg:       cfg.normalized(),
		state:     model.CircuitClosed,
		now:       time.Now,
	}
}

// IsAllowed 判断请求是否允许通过
func (cb *CircuitBreaker) IsAllowed() bool {
	return cb.acquireProbe(true)
}

// PeekAllowed 判断当前是否可能允许请求通过，但不占用 half-open 探测名额。
// 调度层用它过滤候选渠道，避免仅检查候选就污染 halfOpenInFlight。
func (cb *CircuitBreaker) PeekAllowed() bool {
	return cb.acquireProbe(false)
}

func (cb *CircuitBreaker) acquireProbe(reserve bool) bool {
	var health *model.ChannelHealth

	cb.mu.Lock()
	defer func() {
		cb.mu.Unlock()
		if health != nil {
			queuePersist(health)
		}
	}()

	now := cb.currentTime()
	switch cb.state {
	case model.CircuitClosed:
		return true
	case model.CircuitOpen:
		if cb.openedAt == nil || now.Sub(*cb.openedAt) < time.Duration(cb.cfg.TimeoutSeconds)*time.Second {
			return false
		}
		if !reserve {
			// Peek 必须是只读的：调度层会对每个候选渠道调用它，若在这里迁移状态，
			// 一次选渠道扫描就会把根本没被选中的渠道全部翻成 HalfOpen 并触发落库。
			// 熔断已超时说明下一个真正的 reserve 调用可以放行，这里只报告可行性。
			return true
		}
		cb.toHalfOpenLocked(now)
		health = cb.stateSnapshotLocked()
		fallthrough
	case model.CircuitHalfOpen:
		if cb.halfOpenInFlight >= cb.cfg.SuccessThreshold {
			return false
		}
		if reserve {
			cb.halfOpenInFlight++
		}
		return true
	default:
		return true
	}
}

// ReleaseProbe 释放一次 half-open 探测名额。
// 当请求已被放行但随后因客户端取消等原因未记录成功/失败时调用，避免 halfOpenInFlight 泄漏。
func (cb *CircuitBreaker) ReleaseProbe() {
	cb.mu.Lock()
	if cb.state == model.CircuitHalfOpen && cb.halfOpenInFlight > 0 {
		cb.halfOpenInFlight--
	}
	cb.mu.Unlock()
}

// RecordSuccess 记录成功请求
func (cb *CircuitBreaker) RecordSuccess() {
	var health *model.ChannelHealth

	cb.mu.Lock()
	now := cb.currentTime()
	prevState := cb.state

	cb.recordEventLocked(now, false)
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses++
	if cb.state == model.CircuitHalfOpen && cb.halfOpenInFlight > 0 {
		cb.halfOpenInFlight--
	}

	if cb.state == model.CircuitHalfOpen && cb.consecutiveSuccesses >= cb.cfg.SuccessThreshold {
		cb.state = model.CircuitClosed
		cb.openedAt = nil
		cb.halfOpenInFlight = 0
	}

	if prevState != cb.state || cb.totalRequests%10 == 0 {
		health = cb.snapshotLocked(now, true, "")
	}
	cb.mu.Unlock()

	if health != nil {
		queuePersist(health)
	}
}

// RecordFailure 记录失败请求
func (cb *CircuitBreaker) RecordFailure(errMsg string) {
	var health *model.ChannelHealth

	cb.mu.Lock()
	now := cb.currentTime()
	prevState := cb.state

	cb.recordEventLocked(now, true)
	cb.consecutiveSuccesses = 0
	cb.consecutiveFailures++
	if cb.state == model.CircuitHalfOpen && cb.halfOpenInFlight > 0 {
		cb.halfOpenInFlight--
	}

	shouldOpen := false
	if cb.state == model.CircuitHalfOpen {
		shouldOpen = true
	} else if cb.state == model.CircuitClosed {
		if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
			shouldOpen = true
		}
		if cb.totalRequests >= cb.cfg.MinRequests {
			errorRate := float64(cb.failedRequests) / float64(cb.totalRequests)
			if errorRate >= cb.cfg.ErrorRateThreshold {
				shouldOpen = true
			}
		}
	}

	if shouldOpen {
		cb.toOpenLocked(now)
	}

	if prevState != cb.state || cb.totalRequests%10 == 0 {
		health = cb.snapshotLocked(now, false, errMsg)
	}
	cb.mu.Unlock()

	if health != nil {
		queuePersist(health)
	}
}

// recordEventLocked 把一次请求结果计入滑动窗口。
//
// failedRequests 用增量维护：此前每次请求都要全量遍历窗口内事件重算，
// 高 QPS 渠道在 60s 窗口下事件数可达数万，等于每请求在写锁临界区内做一次全扫描。
func (cb *CircuitBreaker) recordEventLocked(now time.Time, failed bool) {
	cb.requestEvents = append(cb.requestEvents, requestEvent{at: now, failed: failed})
	if failed {
		cb.failedRequests++
	}
	cb.pruneEventsLocked(now)
	cb.totalRequests = len(cb.requestEvents)
}

// pruneEventsLocked 剔除窗口外事件，并同步扣减 failedRequests。
func (cb *CircuitBreaker) pruneEventsLocked(now time.Time) {
	window := time.Duration(cb.cfg.WindowSeconds) * time.Second
	if window <= 0 || len(cb.requestEvents) == 0 {
		return
	}
	cutoff := now.Add(-window)
	keepFrom := 0
	for keepFrom < len(cb.requestEvents) && cb.requestEvents[keepFrom].at.Before(cutoff) {
		if cb.requestEvents[keepFrom].failed && cb.failedRequests > 0 {
			cb.failedRequests--
		}
		keepFrom++
	}
	if keepFrom > 0 {
		copy(cb.requestEvents, cb.requestEvents[keepFrom:])
		cb.requestEvents = cb.requestEvents[:len(cb.requestEvents)-keepFrom]
	}
}

// recountWindowLocked 从事件列表全量重算计数。
// 仅用于配置变更等罕见路径；常规请求路径走 recordEventLocked 的增量维护。
func (cb *CircuitBreaker) recountWindowLocked() {
	cb.totalRequests = len(cb.requestEvents)
	cb.failedRequests = 0
	for _, event := range cb.requestEvents {
		if event.failed {
			cb.failedRequests++
		}
	}
}

func (cb *CircuitBreaker) toOpenLocked(now time.Time) {
	openedAt := now
	cb.state = model.CircuitOpen
	cb.openedAt = &openedAt
	cb.consecutiveSuccesses = 0
	cb.halfOpenInFlight = 0
}

func (cb *CircuitBreaker) toHalfOpenLocked(now time.Time) {
	cb.state = model.CircuitHalfOpen
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.halfOpenInFlight = 0
}

func (cb *CircuitBreaker) currentTime() time.Time {
	if cb.now != nil {
		return cb.now()
	}
	return time.Now()
}

// stateSnapshotLocked 生成一份用于持久化的状态快照，并推进持久化版本号。
//
// 版本号必须每次递增：UpsertChannelHealth 依赖它丢弃过期快照，若多次快照共用同一版本，
// 并发落库的先后顺序完全随机，新状态可能被旧快照覆盖。
func (cb *CircuitBreaker) stateSnapshotLocked() *model.ChannelHealth {
	cb.persistVersion++
	health := &model.ChannelHealth{
		ChannelId:            cb.channelID,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		TotalRequests:        cb.totalRequests,
		FailedRequests:       cb.failedRequests,
		CircuitState:         cb.state,
		PersistVersion:       cb.persistVersion,
	}
	if cb.openedAt != nil {
		openedAt := *cb.openedAt
		health.CircuitOpenedAt = &openedAt
	}
	return health
}

func (cb *CircuitBreaker) snapshotLocked(timestamp time.Time, isSuccess bool, errMsg string) *model.ChannelHealth {
	health := cb.stateSnapshotLocked()
	if isSuccess {
		lastSuccess := timestamp
		health.LastSuccessAt = &lastSuccess
	} else {
		lastFailure := timestamp
		health.LastFailureAt = &lastFailure
		health.LastError = errMsg
	}
	return health
}

// 熔断状态落库队列。
//
// 此前每次状态变更都 `go persistHealth(...)`：无数量上限、无排队，
// 高失败率时会瞬间派生大量 goroutine 争抢 SQLite 的单写连接。
// 改为单 goroutine 顺序消费的有界队列；队列满时丢弃本次快照
// （状态会在后续变更或进程重启时从内存/DB 重新对齐，丢一帧快照不影响正确性）。
const persistQueueSize = 256

var (
	persistQueue     chan *model.ChannelHealth
	persistQueueOnce sync.Once
)

func queuePersist(health *model.ChannelHealth) {
	if health == nil {
		return
	}
	persistQueueOnce.Do(func() {
		persistQueue = make(chan *model.ChannelHealth, persistQueueSize)
		go func() {
			for h := range persistQueue {
				persistHealth(h)
			}
		}()
	})
	select {
	case persistQueue <- health:
	default:
		// 队列积压：丢弃最新快照，避免阻塞请求路径。
	}
}

// persistHealth 持久化到数据库
func persistHealth(health *model.ChannelHealth) {
	if health == nil || model.DB == nil {
		return
	}
	_ = model.UpsertChannelHealth(health)
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() model.CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset 重置熔断器状态；仅在持久化状态完整清除后提交内存重置。
func (cb *CircuitBreaker) Reset() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	nextVersion := cb.persistVersion + 1
	if model.DB != nil {
		if err := model.ResetChannelHealth(cb.channelID, nextVersion); err != nil {
			return err
		}
	}

	cb.state = model.CircuitClosed
	cb.openedAt = nil
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.totalRequests = 0
	cb.failedRequests = 0
	cb.requestEvents = nil
	cb.halfOpenInFlight = 0
	cb.persistVersion = nextVersion
	return nil
}
