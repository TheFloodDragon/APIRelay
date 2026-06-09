package controller

import (
	"testing"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
)

func TestCircuitBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	cb := newCircuitBreaker(3, 30*time.Second, func() time.Time { return now })

	if !cb.Allow(1) {
		t.Fatal("new breaker did not allow closed channel")
	}

	cb.RecordFailure(1)
	cb.RecordFailure(1)
	if state := cb.State(1); state != CircuitBreakerClosed {
		t.Fatalf("state after 2 failures = %s, want %s", state, CircuitBreakerClosed)
	}
	if !cb.Allow(1) {
		t.Fatal("breaker opened before threshold")
	}

	cb.RecordFailure(1)
	if state := cb.State(1); state != CircuitBreakerOpen {
		t.Fatalf("state after threshold failures = %s, want %s", state, CircuitBreakerOpen)
	}
	if cb.Allow(1) {
		t.Fatal("open breaker allowed request before expiry")
	}
}

func TestCircuitBreakerIsScopedByChannel(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	cb := newCircuitBreaker(3, 30*time.Second, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		cb.RecordFailure(7)
	}

	if state := cb.State(7); state != CircuitBreakerOpen {
		t.Fatalf("channel 7 state = %s, want %s", state, CircuitBreakerOpen)
	}
	if cb.Allow(7) {
		t.Fatal("open breaker allowed same channel")
	}
	if !cb.Allow(8) {
		t.Fatal("different channel was blocked by channel 7 breaker state")
	}
	if state := cb.State(8); state != CircuitBreakerClosed {
		t.Fatalf("channel 8 state = %s, want %s", state, CircuitBreakerClosed)
	}
}

func TestCircuitBreakerHalfOpenSuccessCloses(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	cb := newCircuitBreaker(3, 30*time.Second, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		cb.RecordFailure(9)
	}
	if state := cb.State(9); state != CircuitBreakerOpen {
		t.Fatalf("initial state = %s, want %s", state, CircuitBreakerOpen)
	}

	now = now.Add(31 * time.Second)
	if !cb.Allow(9) {
		t.Fatal("expired open breaker did not allow half-open probe")
	}
	if state := cb.State(9); state != CircuitBreakerHalfOpen {
		t.Fatalf("state after expiry = %s, want %s", state, CircuitBreakerHalfOpen)
	}

	cb.RecordSuccess(9)
	if state := cb.State(9); state != CircuitBreakerClosed {
		t.Fatalf("state after half-open success = %s, want %s", state, CircuitBreakerClosed)
	}
	if !cb.Allow(9) {
		t.Fatal("closed breaker did not allow request")
	}
}

func TestFilterCircuitOpenCandidates(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	rc := &RelayController{circuitBreaker: newCircuitBreaker(3, 30*time.Second, func() time.Time { return now })}
	candidates := []relayCandidate{
		{Channel: model.Channel{ID: 1}, ResolvedModel: "model-a"},
		{Channel: model.Channel{ID: 2}, ResolvedModel: "model-a"},
	}
	for i := 0; i < 3; i++ {
		rc.circuitBreaker.RecordFailure(1)
	}

	filtered := rc.filterCircuitOpenCandidates(candidates)
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(filtered))
	}
	if filtered[0].Channel.ID != 2 {
		t.Fatalf("filtered channel ID = %d, want 2", filtered[0].Channel.ID)
	}
}

func TestFilterCircuitOpenCandidatesKeepsOriginalWhenAllOpen(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	rc := &RelayController{circuitBreaker: newCircuitBreaker(3, 30*time.Second, func() time.Time { return now })}
	candidates := []relayCandidate{
		{Channel: model.Channel{ID: 1}, ResolvedModel: "model-a"},
		{Channel: model.Channel{ID: 2}, ResolvedModel: "model-a"},
	}
	for _, channelID := range []uint{1, 2} {
		for i := 0; i < 3; i++ {
			rc.circuitBreaker.RecordFailure(channelID)
		}
	}

	filtered := rc.filterCircuitOpenCandidates(candidates)
	if len(filtered) != len(candidates) {
		t.Fatalf("filtered len = %d, want original len %d", len(filtered), len(candidates))
	}
	for i := range candidates {
		if filtered[i].Channel.ID != candidates[i].Channel.ID {
			t.Fatalf("filtered[%d] channel ID = %d, want %d", i, filtered[i].Channel.ID, candidates[i].Channel.ID)
		}
	}
}
