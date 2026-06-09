package controller

import (
	"strconv"
	"sync"
	"time"
)

const (
	defaultCircuitBreakerFailureThreshold = 3
	defaultCircuitBreakerOpenDuration     = 30 * time.Second
)

// CircuitBreakerState 表示单个渠道熔断器的状态。
type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "closed"
	CircuitBreakerOpen     CircuitBreakerState = "open"
	CircuitBreakerHalfOpen CircuitBreakerState = "half_open"
)

type circuitEntry struct {
	state               CircuitBreakerState
	consecutiveFailures int
	openedUntil         time.Time
}

// CircuitBreaker 是进程内、按 channel_id 隔离的轻量熔断器。
type CircuitBreaker struct {
	mu               sync.Mutex
	entries          map[string]*circuitEntry
	failureThreshold int
	openDuration     time.Duration
	now              func() time.Time
}

func NewCircuitBreaker() *CircuitBreaker {
	return newCircuitBreaker(defaultCircuitBreakerFailureThreshold, defaultCircuitBreakerOpenDuration, time.Now)
}

func newCircuitBreaker(failureThreshold int, openDuration time.Duration, now func() time.Time) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = defaultCircuitBreakerFailureThreshold
	}
	if openDuration <= 0 {
		openDuration = defaultCircuitBreakerOpenDuration
	}
	if now == nil {
		now = time.Now
	}
	return &CircuitBreaker{
		entries:          make(map[string]*circuitEntry),
		failureThreshold: failureThreshold,
		openDuration:     openDuration,
		now:              now,
	}
}

func (cb *CircuitBreaker) Allow(channelID uint) bool {
	if cb == nil {
		return true
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	entry := cb.entries[circuitKey(channelID)]
	if entry == nil {
		return true
	}
	cb.advanceOpenIfExpiredLocked(entry, cb.now())
	return entry.state != CircuitBreakerOpen
}

func (cb *CircuitBreaker) RecordSuccess(channelID uint) {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.entries, circuitKey(channelID))
}

func (cb *CircuitBreaker) RecordFailure(channelID uint) {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	key := circuitKey(channelID)
	entry := cb.entries[key]
	if entry == nil {
		entry = &circuitEntry{state: CircuitBreakerClosed}
		cb.entries[key] = entry
	}

	now := cb.now()
	cb.advanceOpenIfExpiredLocked(entry, now)
	switch entry.state {
	case CircuitBreakerHalfOpen, CircuitBreakerOpen:
		cb.openLocked(entry, now)
	default:
		entry.consecutiveFailures++
		if entry.consecutiveFailures >= cb.failureThreshold {
			cb.openLocked(entry, now)
		}
	}
}

func (cb *CircuitBreaker) State(channelID uint) CircuitBreakerState {
	if cb == nil {
		return CircuitBreakerClosed
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	entry := cb.entries[circuitKey(channelID)]
	if entry == nil {
		return CircuitBreakerClosed
	}
	cb.advanceOpenIfExpiredLocked(entry, cb.now())
	return entry.state
}

func (cb *CircuitBreaker) advanceOpenIfExpiredLocked(entry *circuitEntry, now time.Time) {
	if entry.state == CircuitBreakerOpen && !now.Before(entry.openedUntil) {
		entry.state = CircuitBreakerHalfOpen
		entry.consecutiveFailures = 0
	}
}

func (cb *CircuitBreaker) openLocked(entry *circuitEntry, now time.Time) {
	entry.state = CircuitBreakerOpen
	entry.consecutiveFailures = cb.failureThreshold
	entry.openedUntil = now.Add(cb.openDuration)
}

func circuitKey(channelID uint) string {
	return strconv.FormatUint(uint64(channelID), 10)
}
