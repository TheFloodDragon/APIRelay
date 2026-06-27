package circuitbreaker

import (
	"sync"
	"time"

	"github.com/apirelay/apirelay/model"
)

// CircuitBreaker 熔断器实例（per-channel）
type CircuitBreaker struct {
	channelID int
	cfg       Config
	mu        sync.RWMutex
	state     model.CircuitState
	openedAt  *time.Time
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
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == model.CircuitClosed {
		return true
	}
	if cb.state == model.CircuitOpen {
		// 检查是否超过超时时间，可以进入半开状态
		if cb.openedAt != nil && time.Since(*cb.openedAt) > time.Duration(cb.cfg.TimeoutSeconds)*time.Second {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = model.CircuitHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
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

	// 从数据库加载当前状态
	health, err := model.GetChannelHealth(cb.channelID)
	if err != nil {
		return
	}

	now := time.Now()
	health.LastSuccessAt = &now
	health.ConsecutiveFailures = 0
	health.ConsecutiveSuccesses++
	health.TotalRequests++

	// 状态转换逻辑
	if cb.state == model.CircuitHalfOpen && health.ConsecutiveSuccesses >= cb.cfg.SuccessThreshold {
		cb.state = model.CircuitClosed
		health.CircuitState = model.CircuitClosed
		health.CircuitOpenedAt = nil
	} else {
		health.CircuitState = cb.state
	}

	_ = model.UpsertChannelHealth(health)
}

// RecordFailure 记录失败请求
func (cb *CircuitBreaker) RecordFailure(errMsg string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	health, err := model.GetChannelHealth(cb.channelID)
	if err != nil {
		return
	}

	now := time.Now()
	health.LastFailureAt = &now
	health.LastError = errMsg
	health.ConsecutiveSuccesses = 0
	health.ConsecutiveFailures++
	health.TotalRequests++
	health.FailedRequests++

	// 状态转换逻辑
	shouldOpen := false
	if cb.state == model.CircuitHalfOpen {
		// 半开状态任何失败都直接熔断
		shouldOpen = true
	} else if cb.state == model.CircuitClosed {
		// 闭合状态检查是否达到阈值
		if health.ConsecutiveFailures >= cb.cfg.FailureThreshold {
			shouldOpen = true
		}
		// 或检查错误率
		if health.TotalRequests >= cb.cfg.MinRequests {
			errorRate := float64(health.FailedRequests) / float64(health.TotalRequests)
			if errorRate >= cb.cfg.ErrorRateThreshold {
				shouldOpen = true
			}
		}
	}

	if shouldOpen {
		cb.state = model.CircuitOpen
		cb.openedAt = &now
		health.CircuitState = model.CircuitOpen
		health.CircuitOpenedAt = &now
	} else {
		health.CircuitState = cb.state
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
