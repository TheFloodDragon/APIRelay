package circuit

import (
	"testing"
	"time"
)

func TestBreakerOpensAfterFailures(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	breaker := NewBreakerWithClock(3, 1, 30*time.Second, func() time.Time { return now })

	if result := breaker.Allow(1); !result.Allowed || result.State != CircuitStateClosed {
		t.Fatalf("initial allow = %#v, want closed allowed", result)
	}
	breaker.RecordFailure(1, false)
	breaker.RecordFailure(1, false)
	if state := breaker.State(1); state != CircuitStateClosed {
		t.Fatalf("state after 2 failures = %s, want %s", state, CircuitStateClosed)
	}
	breaker.RecordFailure(1, false)
	if state := breaker.State(1); state != CircuitStateOpen {
		t.Fatalf("state after threshold = %s, want %s", state, CircuitStateOpen)
	}
	if result := breaker.Allow(1); result.Allowed {
		t.Fatalf("open breaker allowed request: %#v", result)
	}
}

func TestBreakerHalfOpenPermitAndSuccessThreshold(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	breaker := NewBreakerWithClock(2, 2, 30*time.Second, func() time.Time { return now })

	breaker.RecordFailure(7, false)
	breaker.RecordFailure(7, false)
	now = now.Add(31 * time.Second)

	first := breaker.Allow(7)
	if !first.Allowed || !first.UsedHalfOpenPermit || first.State != CircuitStateHalfOpen {
		t.Fatalf("first half-open allow = %#v", first)
	}
	second := breaker.Allow(7)
	if second.Allowed || second.State != CircuitStateHalfOpen {
		t.Fatalf("second concurrent half-open allow = %#v, want denied", second)
	}
	breaker.RecordSuccess(7, first.UsedHalfOpenPermit)
	if state := breaker.State(7); state != CircuitStateHalfOpen {
		t.Fatalf("state after one success = %s, want %s", state, CircuitStateHalfOpen)
	}

	third := breaker.Allow(7)
	if !third.Allowed || !third.UsedHalfOpenPermit {
		t.Fatalf("third half-open allow = %#v", third)
	}
	breaker.RecordSuccess(7, third.UsedHalfOpenPermit)
	if state := breaker.State(7); state != CircuitStateClosed {
		t.Fatalf("state after success threshold = %s, want %s", state, CircuitStateClosed)
	}
}

func TestBreakerHalfOpenFailureReopens(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	breaker := NewBreakerWithClock(1, 1, 30*time.Second, func() time.Time { return now })

	breaker.RecordFailure(3, false)
	now = now.Add(31 * time.Second)
	permit := breaker.Allow(3)
	if !permit.Allowed || !permit.UsedHalfOpenPermit {
		t.Fatalf("half-open allow = %#v", permit)
	}
	breaker.RecordFailure(3, permit.UsedHalfOpenPermit)
	if state := breaker.State(3); state != CircuitStateOpen {
		t.Fatalf("state after half-open failure = %s, want %s", state, CircuitStateOpen)
	}
}

func TestBreakerReset(t *testing.T) {
	breaker := NewBreaker(1, 1, 30*time.Second)
	breaker.RecordFailure(11, false)
	if state := breaker.State(11); state != CircuitStateOpen {
		t.Fatalf("state = %s, want open", state)
	}
	breaker.Reset(11)
	if state := breaker.State(11); state != CircuitStateClosed {
		t.Fatalf("state after reset = %s, want closed", state)
	}
}

func TestBreakerKeyedOnlyByChannelID(t *testing.T) {
	breaker := NewBreaker(1, 1, 30*time.Second)
	breaker.RecordFailure(21, false)
	if breaker.Allow(21).Allowed {
		t.Fatal("channel 21 should be open")
	}
	if !breaker.Allow(22).Allowed {
		t.Fatal("different channel should not share breaker state")
	}
}
