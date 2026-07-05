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
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := cb.currentTime()
	switch cb.state {
	case model.CircuitClosed:
		return true
	case model.CircuitOpen:
		if cb.openedAt != nil && now.Sub(*cb.openedAt) >= time.Duration(cb.cfg.TimeoutSeconds)*time.Second {
			cb.toHalfOpenLocked(now)
			health := cb.stateSnapshotLocked()
			go persistHealth(health)
			return true
		}
		return false
	case model.CircuitHalfOpen:
		if cb.halfOpenInFlight >= cb.cfg.SuccessThreshold {
			return false
		}
		cb.halfOpenInFlight++
		return true
	default:
		return true
	}
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
		go persistHealth(health)
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
		go persistHealth(health)
	}
}

func (cb *CircuitBreaker) recordEventLocked(now time.Time, failed bool) {
	cb.requestEvents = append(cb.requestEvents, requestEvent{at: now, failed: failed})
	cb.pruneEventsLocked(now)
	cb.totalRequests = len(cb.requestEvents)
	cb.failedRequests = 0
	for _, event := range cb.requestEvents {
		if event.failed {
			cb.failedRequests++
		}
	}
}

func (cb *CircuitBreaker) pruneEventsLocked(now time.Time) {
	window := time.Duration(cb.cfg.WindowSeconds) * time.Second
	if window <= 0 || len(cb.requestEvents) == 0 {
		return
	}
	cutoff := now.Add(-window)
	keepFrom := 0
	for keepFrom < len(cb.requestEvents) && cb.requestEvents[keepFrom].at.Before(cutoff) {
		keepFrom++
	}
	if keepFrom > 0 {
		copy(cb.requestEvents, cb.requestEvents[keepFrom:])
		cb.requestEvents = cb.requestEvents[:len(cb.requestEvents)-keepFrom]
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
	cb.halfOpenInFlight = 1
}

func (cb *CircuitBreaker) currentTime() time.Time {
	if cb.now != nil {
		return cb.now()
	}
	return time.Now()
}

func (cb *CircuitBreaker) stateSnapshotLocked() *model.ChannelHealth {
	health := &model.ChannelHealth{
		ChannelId:            cb.channelID,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		TotalRequests:        cb.totalRequests,
		FailedRequests:       cb.failedRequests,
		CircuitState:         cb.state,
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

// persistHealth 异步持久化到数据库
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

// Reset 重置熔断器状态
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	cb.state = model.CircuitClosed
	cb.openedAt = nil
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses = 0
	cb.totalRequests = 0
	cb.failedRequests = 0
	cb.requestEvents = nil
	cb.halfOpenInFlight = 0
	cb.mu.Unlock()

	if model.DB != nil {
		_ = model.ResetChannelHealth(cb.channelID)
	}
}
