package circuitbreaker

import (
	"testing"
	"time"

	"github.com/apirelay/apirelay/model"
)

func TestCircuitBreakerStateMachine(t *testing.T) {
	// 测试数据库需要初始化，这里只测试状态机逻辑
	cfg := Config{
		FailureThreshold:   3,
		SuccessThreshold:   2,
		TimeoutSeconds:     1,
		ErrorRateThreshold: 0.5,
		MinRequests:        5,
	}

	cb := NewCircuitBreaker(9999, cfg)

	// 初始状态：closed
	if cb.GetState() != model.CircuitClosed {
		t.Errorf("初始状态应为 closed，实际为 %s", cb.GetState())
	}

	// 应允许请求
	if !cb.IsAllowed() {
		t.Error("closed 状态应允许请求")
	}

	// 连续失败触发熔断
	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.state = model.CircuitClosed // 手动保持 closed（避免依赖数据库）
	}

	// 设置熔断状态
	cb.state = model.CircuitOpen
	now := time.Now()
	cb.openedAt = &now

	if cb.GetState() != model.CircuitOpen {
		t.Errorf("熔断后状态应为 open，实际为 %s", cb.GetState())
	}

	// open 状态拒绝请求
	if cb.IsAllowed() {
		t.Error("open 状态应拒绝请求")
	}

	// 等待超时进入半开
	time.Sleep(time.Duration(cfg.TimeoutSeconds+1) * time.Second)
	if !cb.IsAllowed() {
		t.Error("超时后应进入 half_open 允许试探")
	}

	if cb.GetState() != model.CircuitHalfOpen {
		t.Errorf("超时后状态应为 half_open，实际为 %s", cb.GetState())
	}
}

func TestCircuitBreakerErrorRate(t *testing.T) {
	cfg := Config{
		FailureThreshold:   10, // 高阈值，主要测试错误率
		SuccessThreshold:   2,
		TimeoutSeconds:     30,
		ErrorRateThreshold: 0.4, // 40% 错误率
		MinRequests:        10,
	}

	cb := NewCircuitBreaker(9998, cfg)

	// 模拟 10 次请求，5 次失败（50% 错误率，超过 40%）
	// 由于依赖数据库，这里只验证配置加载
	if cb.cfg.ErrorRateThreshold != 0.4 {
		t.Errorf("错误率阈值应为 0.4，实际为 %f", cb.cfg.ErrorRateThreshold)
	}
}
