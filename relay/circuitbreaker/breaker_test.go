package circuitbreaker

import (
	"sync"
	"testing"
	"time"

	"github.com/apirelay/apirelay/model"
)

func testConfig() Config {
	return Config{
		FailureThreshold:   3,
		SuccessThreshold:   2,
		TimeoutSeconds:     1,
		ErrorRateThreshold: 0.5,
		MinRequests:        4,
		WindowSeconds:      10,
	}
}

func TestNormalizeConfig(t *testing.T) {
	cfg := NormalizeConfig(Config{
		FailureThreshold:   -1,
		SuccessThreshold:   0,
		TimeoutSeconds:     -10,
		ErrorRateThreshold: 2,
		MinRequests:        0,
		WindowSeconds:      -30,
		ChannelMaxRetries:  -1,
	})
	def := DefaultConfig()

	if cfg.FailureThreshold != def.FailureThreshold || cfg.SuccessThreshold != def.SuccessThreshold || cfg.TimeoutSeconds != def.TimeoutSeconds {
		t.Fatalf("基础阈值未正确回退默认值: %#v", cfg)
	}
	if cfg.MinRequests != def.MinRequests || cfg.WindowSeconds != def.WindowSeconds || cfg.ChannelMaxRetries != def.ChannelMaxRetries {
		t.Fatalf("窗口/重试配置未正确回退默认值: %#v", cfg)
	}
	if cfg.ErrorRateThreshold != 1 {
		t.Fatalf("错误率阈值应被限制到 1，实际为 %v", cfg.ErrorRateThreshold)
	}

	zeroRetry := NormalizeConfig(Config{
		FailureThreshold:   1,
		SuccessThreshold:   1,
		TimeoutSeconds:     1,
		ErrorRateThreshold: 0.5,
		MinRequests:        1,
		WindowSeconds:      1,
		ChannelMaxRetries:  0,
	})
	if zeroRetry.ChannelMaxRetries != 0 {
		t.Fatalf("0 次单渠道重试应被保留，实际为 %d", zeroRetry.ChannelMaxRetries)
	}
}

func TestCircuitBreakerStateMachine(t *testing.T) {
	cfg := testConfig()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	cb := NewCircuitBreaker(9999, cfg)
	cb.now = func() time.Time { return now }

	if cb.GetState() != model.CircuitClosed {
		t.Fatalf("初始状态应为 closed，实际为 %s", cb.GetState())
	}
	if !cb.IsAllowed() {
		t.Fatal("closed 状态应允许请求")
	}

	for i := 0; i < cfg.FailureThreshold; i++ {
		cb.RecordFailure("temporary upstream failure")
	}
	if cb.GetState() != model.CircuitOpen {
		t.Fatalf("连续失败后状态应为 open，实际为 %s", cb.GetState())
	}
	if cb.IsAllowed() {
		t.Fatal("open 且未超时时应拒绝请求")
	}

	now = base.Add(time.Duration(cfg.TimeoutSeconds) * time.Second)
	if !cb.IsAllowed() {
		t.Fatal("熔断超时后应允许第一个 half_open 试探请求")
	}
	if cb.GetState() != model.CircuitHalfOpen {
		t.Fatalf("超时后状态应为 half_open，实际为 %s", cb.GetState())
	}
	if !cb.IsAllowed() {
		t.Fatal("half_open 应允许 success_threshold 数量内的试探请求")
	}
	if cb.IsAllowed() {
		t.Fatal("half_open 达到试探上限后应暂时拒绝额外请求")
	}

	cb.RecordSuccess()
	if cb.GetState() != model.CircuitHalfOpen {
		t.Fatalf("未达到恢复成功阈值前应保持 half_open，实际为 %s", cb.GetState())
	}
	cb.RecordSuccess()
	if cb.GetState() != model.CircuitClosed {
		t.Fatalf("连续试探成功后应恢复 closed，实际为 %s", cb.GetState())
	}
}

func TestCircuitBreakerOpensOnWindowErrorRate(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 100
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	cb := NewCircuitBreaker(9998, cfg)
	cb.now = func() time.Time { return now }

	cb.RecordSuccess()
	cb.RecordFailure("first failure")
	cb.RecordSuccess()
	if cb.GetState() != model.CircuitClosed {
		t.Fatalf("未达到 min_requests 前不应按错误率熔断，实际为 %s", cb.GetState())
	}
	cb.RecordFailure("second failure")

	if cb.GetState() != model.CircuitOpen {
		t.Fatalf("窗口内错误率达到阈值后应熔断，实际为 %s", cb.GetState())
	}
}

func TestCircuitBreakerSlidingWindowExpiresOldFailures(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 100
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	cb := NewCircuitBreaker(9997, cfg)
	cb.now = func() time.Time { return now }

	cb.RecordFailure("old failure 1")
	cb.RecordFailure("old failure 2")
	cb.RecordFailure("old failure 3")

	now = base.Add(time.Duration(cfg.WindowSeconds+1) * time.Second)
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordSuccess()
	cb.RecordFailure("fresh failure")

	if cb.GetState() != model.CircuitClosed {
		t.Fatalf("过期失败不应参与当前窗口错误率，实际状态为 %s", cb.GetState())
	}
	cb.mu.RLock()
	total, failed := cb.totalRequests, cb.failedRequests
	cb.mu.RUnlock()
	if total != 4 || failed != 1 {
		t.Fatalf("窗口计数 = total:%d failed:%d，期望 total:4 failed:1", total, failed)
	}
}

func TestCircuitBreakerHalfOpenFailureReopens(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cfg.SuccessThreshold = 1
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	cb := NewCircuitBreaker(9996, cfg)
	cb.now = func() time.Time { return now }

	cb.RecordFailure("open immediately")
	if cb.GetState() != model.CircuitOpen {
		t.Fatalf("应已熔断，实际为 %s", cb.GetState())
	}
	now = base.Add(time.Duration(cfg.TimeoutSeconds) * time.Second)
	if !cb.IsAllowed() {
		t.Fatal("超时后应允许 half_open 试探")
	}
	if cb.IsAllowed() {
		t.Fatal("success_threshold=1 时 half_open 只能有一个试探请求在飞")
	}

	cb.RecordFailure("probe failed")
	if cb.GetState() != model.CircuitOpen {
		t.Fatalf("half_open 试探失败应重新 open，实际为 %s", cb.GetState())
	}
}

func TestCircuitBreakerReleaseProbe(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cfg.SuccessThreshold = 1
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	cb := NewCircuitBreaker(9993, cfg)
	cb.now = func() time.Time { return now }

	cb.RecordFailure("open immediately")
	now = base.Add(time.Duration(cfg.TimeoutSeconds) * time.Second)
	if !cb.IsAllowed() {
		t.Fatal("超时后应允许第一个 half_open 试探")
	}
	if cb.IsAllowed() {
		t.Fatal("探测名额被占用时不应允许第二个请求")
	}
	cb.ReleaseProbe()
	if !cb.IsAllowed() {
		t.Fatal("释放探测名额后应允许新的 half_open 试探")
	}
}

func TestCircuitBreakerConcurrentRecords(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1000
	cfg.MinRequests = 1000
	cfg.ErrorRateThreshold = 1
	cb := NewCircuitBreaker(9995, cfg)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure("concurrent failure")
			}
		}()
	}
	wg.Wait()

	cb.mu.RLock()
	total, failed, state := cb.totalRequests, cb.failedRequests, cb.state
	cb.mu.RUnlock()
	if total != n || failed != n/2 {
		t.Fatalf("并发窗口计数 = total:%d failed:%d，期望 total:%d failed:%d", total, failed, n, n/2)
	}
	if state != model.CircuitClosed {
		t.Fatalf("高阈值并发记录后应保持 closed，实际为 %s", state)
	}
}

func TestCircuitBreakerResetClearsRuntimeCounters(t *testing.T) {
	cfg := testConfig()
	cfg.FailureThreshold = 1
	cb := NewCircuitBreaker(9994, cfg)
	cb.RecordFailure("open")
	if cb.GetState() != model.CircuitOpen {
		t.Fatalf("应已熔断，实际为 %s", cb.GetState())
	}

	cb.Reset()
	cb.mu.RLock()
	state := cb.state
	total := cb.totalRequests
	failed := cb.failedRequests
	events := len(cb.requestEvents)
	cb.mu.RUnlock()

	if state != model.CircuitClosed || total != 0 || failed != 0 || events != 0 {
		t.Fatalf("reset 后状态/计数不正确: state=%s total=%d failed=%d events=%d", state, total, failed, events)
	}
}
