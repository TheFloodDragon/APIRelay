package circuitbreaker

import (
	"sync"

	"github.com/apirelay/apirelay/model"
)

// Manager 管理所有渠道的熔断器实例
type Manager struct {
	cfg      Config
	breakers sync.Map // map[int]*CircuitBreaker
}

var globalManager *Manager
var once sync.Once

// InitManager 初始化全局熔断器管理器
func InitManager(cfg Config) {
	once.Do(func() {
		globalManager = &Manager{cfg: cfg.normalized()}
	})
}

// GetManager 获取全局熔断器管理器
func GetManager() *Manager {
	if globalManager == nil {
		InitManager(DefaultConfig())
	}
	return globalManager
}

// GetBreaker 获取指定渠道的熔断器实例（懒加载）
func (m *Manager) GetBreaker(channelID int) *CircuitBreaker {
	if v, ok := m.breakers.Load(channelID); ok {
		return v.(*CircuitBreaker)
	}

	// 从数据库加载状态初始化
	health, err := model.GetChannelHealth(channelID)
	if err != nil {
		health = &model.ChannelHealth{
			ChannelId:    channelID,
			CircuitState: model.CircuitClosed,
		}
	}

	breaker := NewCircuitBreaker(channelID, m.cfg)
	breaker.state = health.CircuitState
	breaker.openedAt = health.CircuitOpenedAt
	breaker.consecutiveFailures = health.ConsecutiveFailures
	breaker.consecutiveSuccesses = health.ConsecutiveSuccesses
	breaker.totalRequests = health.TotalRequests
	breaker.failedRequests = health.FailedRequests

	actual, _ := m.breakers.LoadOrStore(channelID, breaker)
	return actual.(*CircuitBreaker)
}

// UpdateConfig 更新全局配置（需重新加载所有熔断器）
func (m *Manager) UpdateConfig(cfg Config) {
	cfg = cfg.normalized()
	m.cfg = cfg
	m.breakers.Range(func(key, value interface{}) bool {
		breaker := value.(*CircuitBreaker)
		breaker.mu.Lock()
		breaker.cfg = cfg
		breaker.pruneEventsLocked(breaker.currentTime())
		breaker.totalRequests = len(breaker.requestEvents)
		breaker.failedRequests = 0
		for _, event := range breaker.requestEvents {
			if event.failed {
				breaker.failedRequests++
			}
		}
		breaker.mu.Unlock()
		return true
	})
}

// IsChannelAllowed 判断渠道是否允许请求
func (m *Manager) IsChannelAllowed(channelID int) bool {
	return m.GetBreaker(channelID).IsAllowed()
}

// RecordSuccess 记录渠道成功
func (m *Manager) RecordSuccess(channelID int) {
	m.GetBreaker(channelID).RecordSuccess()
}

// RecordFailure 记录渠道失败
func (m *Manager) RecordFailure(channelID int, errMsg string) {
	m.GetBreaker(channelID).RecordFailure(errMsg)
}

// ResetBreaker 重置渠道熔断器
func (m *Manager) ResetBreaker(channelID int) {
	m.GetBreaker(channelID).Reset()
}
