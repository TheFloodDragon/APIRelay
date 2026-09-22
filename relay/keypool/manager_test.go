package keypool

import (
	"sync"
	"testing"
	"time"

	"github.com/apirelay/apirelay/model"
)

// newTestManager 构造一个隔离的 Manager（不走全局单例），便于并发与配置独立测试。
func newTestManager(cfg Config) *Manager {
	return &Manager{cfg: cfg.normalized()}
}

func keysFor(channelID int, ids ...int) []*model.ChannelKey {
	list := make([]*model.ChannelKey, 0, len(ids))
	for i, id := range ids {
		list = append(list, &model.ChannelKey{
			Id:        id,
			ChannelId: channelID,
			KeyIndex:  i,
			Key:       "sk-test",
			Status:    model.ChannelKeyStatusEnabled,
			Available: true,
		})
	}
	return list
}

func TestSelectKeySequentialPrefersLowestIndex(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10, 11, 12)
	now := time.Now().UnixMilli()

	got := m.SelectKey(keys, nil, now)
	if got == nil || got.Id != 10 {
		t.Fatalf("expected first key (id=10), got %+v", got)
	}
}

func TestSelectKeySkipsExcluded(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10, 11, 12)
	now := time.Now().UnixMilli()

	got := m.SelectKey(keys, map[int]struct{}{10: {}}, now)
	if got == nil || got.Id != 11 {
		t.Fatalf("expected id=11 after excluding 10, got %+v", got)
	}
}

func TestSelectKeySkipsCooldownAndDisabled(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10, 11, 12)
	now := time.Now().UnixMilli()

	// id=10 限流冷却；id=11 鉴权失效；应选 id=12。
	m.RecordFailure(keys[0], model.DisableReasonRateLimited, 0, "429")
	m.RecordFailure(keys[1], model.DisableReasonAuthFailed, 0, "401")

	got := m.SelectKey(keys, nil, now)
	if got == nil || got.Id != 12 {
		t.Fatalf("expected id=12 (others cooled/disabled), got %+v", got)
	}
}

func TestSelectKeyReturnsNilWhenAllUnavailable(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10, 11)
	m.RecordFailure(keys[0], model.DisableReasonAuthFailed, 0, "401")
	m.RecordFailure(keys[1], model.DisableReasonAuthFailed, 0, "401")

	if got := m.SelectKey(keys, nil, time.Now().UnixMilli()); got != nil {
		t.Fatalf("expected nil when all disabled, got %+v", got)
	}
}

func TestRateLimitedUsesRetryAfter(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10)
	m.RecordFailure(keys[0], model.DisableReasonRateLimited, 2*time.Second, "429")

	now := time.Now().UnixMilli()
	// 冷却期内不可用。
	if got := m.SelectKey(keys, nil, now); got != nil {
		t.Fatalf("expected key cooled down, got %+v", got)
	}
	// retry-after 之后恢复可用（被动恢复）。
	if got := m.SelectKey(keys, nil, now+3000); got == nil {
		t.Fatal("expected key available after retry-after window")
	}
}

func TestAuthFailDisablesHard(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10)
	disabled := m.RecordFailure(keys[0], model.DisableReasonAuthFailed, 0, "401 unauthorized")
	if !disabled {
		t.Fatal("auth failure with AuthFailDisable=true should hard-disable")
	}
	// 即使很久以后也不会被动恢复（需主动探测）。
	if m.candidateAvailable(keys[0], nil, time.Now().UnixMilli()+3_600_000) {
		t.Fatal("hard-disabled key must not passively recover")
	}
}

func TestAuthFailCooldownWhenDisableOff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuthFailDisable = false
	cfg.CooldownSeconds = 1
	m := newTestManager(cfg)
	keys := keysFor(1, 10)

	disabled := m.RecordFailure(keys[0], model.DisableReasonAuthFailed, 0, "401")
	if disabled {
		t.Fatal("AuthFailDisable=false should cool down, not hard-disable")
	}
	now := time.Now().UnixMilli()
	if m.candidateAvailable(keys[0], nil, now) {
		t.Fatal("should be cooling down right after failure")
	}
	if !m.candidateAvailable(keys[0], nil, now+2000) {
		t.Fatal("should recover after cooldown window")
	}
}

func TestUpstreamErrorDisablesAfterThreshold(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 3
	m := newTestManager(cfg)
	keys := keysFor(1, 10)

	var disabled bool
	for i := 0; i < 3; i++ {
		disabled = m.RecordFailure(keys[0], model.DisableReasonUpstreamError, 0, "502")
	}
	if !disabled {
		t.Fatal("consecutive upstream errors should hard-disable at threshold")
	}
}

func TestRecordSuccessResetsState(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 2
	m := newTestManager(cfg)
	keys := keysFor(1, 10)

	m.RecordFailure(keys[0], model.DisableReasonUpstreamError, 0, "502")
	m.RecordSuccess(keys[0])

	// 成功后连续失败清零：再来一次不应立即失效。
	disabled := m.RecordFailure(keys[0], model.DisableReasonUpstreamError, 0, "502")
	if disabled {
		t.Fatal("consecutive failures should have been reset by success")
	}
}

func TestMarkRecoveredReenablesKey(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10)
	m.RecordFailure(keys[0], model.DisableReasonAuthFailed, 0, "401")
	if m.candidateAvailable(keys[0], nil, time.Now().UnixMilli()) {
		t.Fatal("key should be disabled before recovery")
	}
	m.MarkRecovered(keys[0])
	if !m.candidateAvailable(keys[0], nil, time.Now().UnixMilli()) {
		t.Fatal("key should be available after MarkRecovered")
	}
}

func TestVirtualKeyAlwaysAvailable(t *testing.T) {
	m := newTestManager(DefaultConfig())
	virtual := &model.ChannelKey{Id: 0, ChannelId: 1, Key: "sk-legacy", Available: true}
	keys := []*model.ChannelKey{virtual}

	// 对虚拟 Key 记录失败应是无操作（健康交给渠道级）。
	if disabled := m.RecordFailure(virtual, model.DisableReasonAuthFailed, 0, "401"); disabled {
		t.Fatal("virtual key must not be tracked/disabled by keypool")
	}
	if got := m.SelectKey(keys, nil, time.Now().UnixMilli()); got == nil {
		t.Fatal("virtual key should always be selectable")
	}
	// 但仍受 excluded 约束。
	if got := m.SelectKey(keys, map[int]struct{}{0: {}}, time.Now().UnixMilli()); got != nil {
		t.Fatal("excluded virtual key should be skipped")
	}
}

func TestSnapshotReflectsState(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10)
	m.RecordFailure(keys[0], model.DisableReasonQuotaExhausted, 0, "insufficient_quota")

	rt, ok := m.Snapshot(10)
	if !ok {
		t.Fatal("expected snapshot to exist")
	}
	if rt.Available {
		t.Fatal("snapshot should reflect disabled state")
	}
	if rt.DisabledReason != model.DisableReasonQuotaExhausted {
		t.Fatalf("disabled reason = %q, want quota_exhausted", rt.DisabledReason)
	}
}

func TestRoundRobinRotates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Strategy = StrategyRoundRobin
	m := newTestManager(cfg)
	keys := keysFor(1, 10, 11, 12)
	now := time.Now().UnixMilli()

	seen := map[int]int{}
	for i := 0; i < 6; i++ {
		got := m.SelectKey(keys, nil, now)
		if got == nil {
			t.Fatal("round robin returned nil")
		}
		seen[got.Id]++
	}
	// 6 次请求应大致均匀轮转到 3 个 Key，每个至少命中一次。
	for _, id := range []int{10, 11, 12} {
		if seen[id] == 0 {
			t.Fatalf("round robin never selected key %d: %v", id, seen)
		}
	}
}

// TestConcurrentAccess 覆盖 -race：并发选择与状态记录。
func TestConcurrentAccess(t *testing.T) {
	m := newTestManager(DefaultConfig())
	keys := keysFor(1, 10, 11, 12, 13, 14)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			now := time.Now().UnixMilli()
			k := m.SelectKey(keys, nil, now)
			if k == nil {
				return
			}
			if i%2 == 0 {
				m.RecordFailure(k, model.DisableReasonRateLimited, 0, "429")
			} else {
				m.RecordSuccess(k)
			}
			_, _ = m.Snapshot(k.Id)
		}(i)
	}
	wg.Wait()
}
