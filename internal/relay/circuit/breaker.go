package circuit

import (
	"sort"
	"sync"
	"time"
)

const (
	defaultFailureThreshold = 3
	defaultSuccessThreshold = 1
	defaultOpenDuration     = 30 * time.Second
)

type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

type AllowResult struct {
	Allowed            bool
	State              CircuitState
	UsedHalfOpenPermit bool
}

type Snapshot struct {
	ChannelID            uint         `json:"channel_id"`
	State                CircuitState `json:"state"`
	ConsecutiveFailures  int          `json:"consecutive_failures"`
	ConsecutiveSuccesses int          `json:"consecutive_successes"`
	OpenedUntil          *time.Time   `json:"opened_until,omitempty"`
	HalfOpenPermitInUse  bool         `json:"half_open_permit_in_use"`
	FailureThreshold     int          `json:"failure_threshold"`
	SuccessThreshold     int          `json:"success_threshold"`
	OpenDurationSeconds  int          `json:"open_duration_seconds"`
}

type Breaker struct {
	mu               sync.Mutex
	entries          map[uint]*entry
	failureThreshold int
	successThreshold int
	openDuration     time.Duration
	now              func() time.Time
}

type entry struct {
	state                CircuitState
	consecutiveFailures  int
	consecutiveSuccesses int
	openedUntil          time.Time
	halfOpenPermitInUse  bool
}

func NewBreaker(failureThreshold, successThreshold int, openDuration time.Duration) *Breaker {
	return NewBreakerWithClock(failureThreshold, successThreshold, openDuration, time.Now)
}

func NewBreakerWithClock(failureThreshold, successThreshold int, openDuration time.Duration, now func() time.Time) *Breaker {
	failureThreshold, successThreshold, openDuration = normalizeConfig(failureThreshold, successThreshold, openDuration)
	if now == nil {
		now = time.Now
	}
	return &Breaker{
		entries:          make(map[uint]*entry),
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		openDuration:     openDuration,
		now:              now,
	}
}

func (b *Breaker) Allow(channelID uint) AllowResult {
	if b == nil {
		return AllowResult{Allowed: true, State: CircuitStateClosed}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	e := b.entryFor(channelID)
	b.advanceOpenIfExpiredLocked(e, b.now())
	switch e.state {
	case CircuitStateOpen:
		return AllowResult{Allowed: false, State: CircuitStateOpen}
	case CircuitStateHalfOpen:
		if e.halfOpenPermitInUse {
			return AllowResult{Allowed: false, State: CircuitStateHalfOpen}
		}
		e.halfOpenPermitInUse = true
		return AllowResult{Allowed: true, State: CircuitStateHalfOpen, UsedHalfOpenPermit: true}
	default:
		return AllowResult{Allowed: true, State: CircuitStateClosed}
	}
}

func (b *Breaker) RecordSuccess(channelID uint, usedHalfOpenPermit bool) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	e := b.entryFor(channelID)
	if usedHalfOpenPermit || e.state == CircuitStateHalfOpen {
		e.halfOpenPermitInUse = false
		e.consecutiveSuccesses++
		if e.consecutiveSuccesses >= b.successThreshold {
			delete(b.entries, channelID)
		}
		return
	}
	delete(b.entries, channelID)
}

func (b *Breaker) RecordFailure(channelID uint, usedHalfOpenPermit bool) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	e := b.entryFor(channelID)
	now := b.now()
	b.advanceOpenIfExpiredLocked(e, now)
	if usedHalfOpenPermit || e.state == CircuitStateHalfOpen || e.state == CircuitStateOpen {
		e.halfOpenPermitInUse = false
		b.openLocked(e, now)
		return
	}
	e.consecutiveFailures++
	if e.consecutiveFailures >= b.failureThreshold {
		b.openLocked(e, now)
	}
}

func (b *Breaker) State(channelID uint) CircuitState {
	if b == nil {
		return CircuitStateClosed
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	e := b.entries[channelID]
	if e == nil {
		return CircuitStateClosed
	}
	b.advanceOpenIfExpiredLocked(e, b.now())
	return e.state
}

func (b *Breaker) Reset(channelID uint) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, channelID)
}

func (b *Breaker) Reconfigure(failureThreshold, successThreshold int, openDuration time.Duration) {
	if b == nil {
		return
	}
	failureThreshold, successThreshold, openDuration = normalizeConfig(failureThreshold, successThreshold, openDuration)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureThreshold = failureThreshold
	b.successThreshold = successThreshold
	b.openDuration = openDuration
}

func (b *Breaker) Snapshot(channelIDs []uint) []Snapshot {
	if b == nil {
		return closedSnapshots(channelIDs)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ids := append([]uint(nil), channelIDs...)
	if len(ids) == 0 {
		ids = make([]uint, 0, len(b.entries))
		for channelID := range b.entries {
			ids = append(ids, channelID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	}

	now := b.now()
	snapshots := make([]Snapshot, 0, len(ids))
	for _, channelID := range ids {
		e := b.entries[channelID]
		if e == nil {
			snapshots = append(snapshots, b.closedSnapshot(channelID))
			continue
		}
		b.advanceOpenIfExpiredLocked(e, now)
		snapshot := b.snapshotForEntry(channelID, e)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func (b *Breaker) entryFor(channelID uint) *entry {
	e := b.entries[channelID]
	if e == nil {
		e = &entry{state: CircuitStateClosed}
		b.entries[channelID] = e
	}
	return e
}

func (b *Breaker) advanceOpenIfExpiredLocked(e *entry, now time.Time) {
	if e.state == CircuitStateOpen && !now.Before(e.openedUntil) {
		e.state = CircuitStateHalfOpen
		e.consecutiveFailures = 0
		e.consecutiveSuccesses = 0
		e.halfOpenPermitInUse = false
	}
}

func (b *Breaker) openLocked(e *entry, now time.Time) {
	e.state = CircuitStateOpen
	e.consecutiveFailures = b.failureThreshold
	e.consecutiveSuccesses = 0
	e.halfOpenPermitInUse = false
	e.openedUntil = now.Add(b.openDuration)
}

func (b *Breaker) closedSnapshot(channelID uint) Snapshot {
	return Snapshot{
		ChannelID:           channelID,
		State:               CircuitStateClosed,
		FailureThreshold:    b.failureThreshold,
		SuccessThreshold:    b.successThreshold,
		OpenDurationSeconds: int(b.openDuration / time.Second),
	}
}

func (b *Breaker) snapshotForEntry(channelID uint, e *entry) Snapshot {
	snapshot := Snapshot{
		ChannelID:            channelID,
		State:                e.state,
		ConsecutiveFailures:  e.consecutiveFailures,
		ConsecutiveSuccesses: e.consecutiveSuccesses,
		HalfOpenPermitInUse:  e.halfOpenPermitInUse,
		FailureThreshold:     b.failureThreshold,
		SuccessThreshold:     b.successThreshold,
		OpenDurationSeconds:  int(b.openDuration / time.Second),
	}
	if e.state == CircuitStateOpen && !e.openedUntil.IsZero() {
		openedUntil := e.openedUntil
		snapshot.OpenedUntil = &openedUntil
	}
	return snapshot
}

func closedSnapshots(channelIDs []uint) []Snapshot {
	snapshots := make([]Snapshot, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		snapshots = append(snapshots, Snapshot{ChannelID: channelID, State: CircuitStateClosed})
	}
	return snapshots
}

func normalizeConfig(failureThreshold, successThreshold int, openDuration time.Duration) (int, int, time.Duration) {
	if failureThreshold <= 0 {
		failureThreshold = defaultFailureThreshold
	}
	if successThreshold <= 0 {
		successThreshold = defaultSuccessThreshold
	}
	if openDuration <= 0 {
		openDuration = defaultOpenDuration
	}
	return failureThreshold, successThreshold, openDuration
}
