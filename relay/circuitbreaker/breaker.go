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
}

// NewCircuitBreaker 创建新的熔断器实例
func NewCircuitBreaker(channelID int, cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		channelID: channelID,
		cfg:       cfg,
		state:     model.CircuitClosed,
	}
}

// IsAllowed 判断请求是否允许通过
func (cb *CircuitBreaker) IsAllowed() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == model.CircuitClosed {
		return true
	}
	if cb.state == model.CircuitOpen {
		// 检查是否超过超时时间，可以进入半开状态
		if cb.openedAt != nil && time.Since(*cb.openedAt) > time.Duration(cb.cfg.TimeoutSeconds)*time.Second {
			cb.state = model.CircuitHalfOpen
			return true
		}
		return false
	}
	// half_open 状态允许少量请求试探
	return true
}

// RecordSuccess 记录成功请求
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.consecutiveFailures = 0
	cb.consecutiveSuccesses++
	cb.totalRequests++

	// 状态转换逻辑
	prevState := cb.state
	if cb.state == model.CircuitHalfOpen && cb.consecutiveSuccesses >= cb.cfg.SuccessThreshold {
		cb.state = model.CircuitClosed
		cb.openedAt = nil
	}

	// 异步持久化
	if prevState != cb.state || cb.totalRequests%10 == 0 {
		go cb.persist(now, true, "")
	}
}

// RecordFailure 记录失败请求
func (cb *CircuitBreaker) RecordFailure(errMsg string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.consecutiveSuccesses = 0
	cb.consecutiveFailures++
	cb.totalRequests++
	cb.failedRequests++

	// 状态转换逻辑
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

	prevState := cb.state
	if shouldOpen {
		cb.state = model.CircuitOpen
		cb.openedAt = &now
	}

	// 异步持久化（状态变化或每10次请求）
	if prevState != cb.state || cb.totalRequests%10 == 0 {
		go cb.persist(now, false, errMsg)
	}
}

// persist 异步持久化到数据库
func (cb *CircuitBreaker) persist(timestamp time.Time, isSuccess bool, errMsg string) {
	health := &model.ChannelHealth{
		ChannelId:            cb.channelID,
		ConsecutiveFailures:  cb.consecutiveFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		TotalRequests:        cb.totalRequests,
		FailedRequests:       cb.failedRequests,
		CircuitState:         cb.state,
		CircuitOpenedAt:      cb.openedAt,
	}
	if isSuccess {
		health.LastSuccessAt = &timestamp
	} else {
		health.LastFailureAt = &timestamp
		health.LastError = errMsg
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
	defer cb.mu.Unlock()
	cb.state = model.CircuitClosed
	cb.openedAt = nil
	_ = model.ResetChannelHealth(cb.channelID)
}
