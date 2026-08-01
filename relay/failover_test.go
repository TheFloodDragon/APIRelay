package relay

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

func TestFailoverState_FatalOnNonRetryable(t *testing.T) {
	s := NewFailoverState(60, 2)
	d := s.OnFailure(1, http.StatusBadRequest, false, "bad request")
	if d != DecisionFatal {
		t.Fatalf("expected fatal, got %d", d)
	}
}

func TestFailoverState_SameChannelThenSwitch(t *testing.T) {
	s := NewFailoverState(60, 2)
	// 429 是瞬时错误，前 maxSameChannelRetries 次应同渠道重试
	for i := 0; i < defaultMaxSameChannelRetries; i++ {
		d := s.OnFailure(7, http.StatusTooManyRequests, true, "rate limited")
		if d != DecisionRetrySameChannel {
			t.Fatalf("retry %d: expected same-channel retry, got %d", i, d)
		}
	}
	// 超过上限后应切换渠道并冷却排除
	d := s.OnFailure(7, http.StatusTooManyRequests, true, "rate limited")
	if d != DecisionSwitchChannel {
		t.Fatalf("expected switch, got %d", d)
	}
	if _, excluded := s.Excluded()[7]; !excluded {
		t.Fatal("channel 7 should be excluded after switch")
	}
}

// setupFailoverDB 建一个带渠道记录的内存库，用于断言冷却时长。
func setupFailoverDB(t *testing.T) *model.Channel {
	t.Helper()
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM channels")
	model.DB.Exec("DELETE FROM abilities")
	ch := &model.Channel{
		Name: "cooldown-target", Type: 1, Status: model.ChannelStatusEnabled,
		BaseURL: "https://example.test", Key: "k", Group: "default", Weight: 1,
		ModelConfigs: `[{"name":"m","enabled":true}]`,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return ch
}

func storedCooldown(t *testing.T, id int) int64 {
	t.Helper()
	saved, err := model.GetChannelByID(id)
	if err != nil {
		t.Fatalf("load channel: %v", err)
	}
	return saved.CooldownUntil
}

// 上游明确告知恢复时刻时，应用它而不是配置的固定冷却，
// 并跳过同渠道重试——上游已说要等，立刻重试必然再次 429。
func TestFailoverState_UsesUpstreamRetryAfterInsteadOfFixedCooldown(t *testing.T) {
	ch := setupFailoverDB(t)
	// 固定冷却设成 3600s，若实现回退到它，断言会明显失败。
	s := NewFailoverState(3600, 2)

	before := time.Now()
	d := s.OnFailureWithHint(ch.Id, http.StatusTooManyRequests, true, "rate limited", 20*time.Second)
	if d != DecisionSwitchChannel {
		t.Fatalf("decision = %d, want switch (upstream asked us to wait)", d)
	}
	if _, excluded := s.Excluded()[ch.Id]; !excluded {
		t.Fatal("channel should be excluded")
	}

	cooldown := storedCooldown(t, ch.Id)
	wantLow := before.Add(19 * time.Second).UnixMilli()
	wantHigh := before.Add(25 * time.Second).UnixMilli()
	if cooldown < wantLow || cooldown > wantHigh {
		t.Fatalf("cooldown_until = %d, want within [%d, %d] (≈20s from now, not the 3600s default)",
			cooldown, wantLow, wantHigh)
	}
}

// 没有解析到恢复时刻时保持原有行为：先同渠道重试，耗尽后按固定冷却切换。
func TestFailoverState_FallsBackToFixedCooldownWithoutHint(t *testing.T) {
	ch := setupFailoverDB(t)
	s := NewFailoverState(30, 1)

	if d := s.OnFailureWithHint(ch.Id, http.StatusTooManyRequests, true, "rate limited", 0); d != DecisionRetrySameChannel {
		t.Fatalf("decision = %d, want same-channel retry when no hint is present", d)
	}

	before := time.Now()
	if d := s.OnFailureWithHint(ch.Id, http.StatusTooManyRequests, true, "rate limited", 0); d != DecisionSwitchChannel {
		t.Fatal("second failure should switch channels")
	}
	cooldown := storedCooldown(t, ch.Id)
	wantLow := before.Add(29 * time.Second).UnixMilli()
	wantHigh := before.Add(35 * time.Second).UnixMilli()
	if cooldown < wantLow || cooldown > wantHigh {
		t.Fatalf("cooldown_until = %d, want ≈30s from now (the configured default)", cooldown)
	}
}

// 不可重试的错误不受 hint 影响，仍应直接判定为 fatal。
func TestFailoverState_HintDoesNotOverrideFatal(t *testing.T) {
	s := NewFailoverState(60, 2)
	d := s.OnFailureWithHint(1, http.StatusBadRequest, false, "bad request", 30*time.Second)
	if d != DecisionFatal {
		t.Fatalf("decision = %d, want fatal", d)
	}
	if _, excluded := s.Excluded()[1]; excluded {
		t.Fatal("fatal errors should not cool down the channel")
	}
}

func TestFailoverState_NonTransientRetryableSwitches(t *testing.T) {
	s := NewFailoverState(60, 2)
	// 502 可重试但非"瞬时"类别，应直接切换渠道而非同渠道重试
	d := s.OnFailure(3, http.StatusBadGateway, true, "bad gateway")
	if d != DecisionSwitchChannel {
		t.Fatalf("expected switch for 502, got %d", d)
	}
}

func TestFailoverState_ZeroSameChannelRetriesSwitches(t *testing.T) {
	s := NewFailoverState(60, 0)
	d := s.OnFailure(7, http.StatusTooManyRequests, true, "rate limited")
	if d != DecisionSwitchChannel {
		t.Fatalf("zero same-channel retries should switch immediately, got %d", d)
	}
	if retries := s.SameChannelRetries[7]; retries != 0 {
		t.Fatalf("same-channel retry counter = %d", retries)
	}
}

func TestRelayerRuntimeChannelMaxRetriesOverride(t *testing.T) {
	relaycommon.SetRuntimeChannelMaxRetries(-1)
	t.Cleanup(func() { relaycommon.SetRuntimeChannelMaxRetries(-1) })

	r := NewRelayer(&config.RelayConfig{ChannelMaxRetries: 2})
	if got := r.channelMaxRetries(); got != 2 {
		t.Fatalf("initial channel max retries = %d", got)
	}

	relaycommon.SetRuntimeChannelMaxRetries(0)
	if got := r.channelMaxRetries(); got != 0 {
		t.Fatalf("runtime channel max retries = %d", got)
	}
}

func TestWeightedPick_SingleAndDistribution(t *testing.T) {
	// 单候选直接返回
	one := []model.ChannelCandidate{{Channel: &model.Channel{Id: 1}, Weight: 0}}
	if weightedPick(one).Id != 1 {
		t.Fatal("single candidate should be returned")
	}

	// 权重为 0 的候选也应有机会被选中（weight+1）
	tier := []model.ChannelCandidate{
		{Channel: &model.Channel{Id: 10}, Weight: 0},
		{Channel: &model.Channel{Id: 20}, Weight: 0},
	}
	seen := map[int]int{}
	for i := 0; i < 200; i++ {
		seen[weightedPick(tier).Id]++
	}
	if seen[10] == 0 || seen[20] == 0 {
		t.Fatalf("both channels should be picked at least once, got %v", seen)
	}
}

func TestFailoverState_RecordAttemptChainJSON(t *testing.T) {
	s := NewFailoverState(60, 1)
	s.RecordAttempt(FailoverAttempt{
		Iter:          0,
		Switches:      0,
		ChannelId:     7,
		ChannelName:   "primary",
		ApiType:       "OpenAI",
		OriginModel:   "gpt-4o",
		UpstreamModel: "gpt-4o-real",
		Status:        http.StatusBadGateway,
		Retryable:     true,
		Decision:      "switch_channel",
		ErrorCategory: string(ErrorCategoryUpstream),
		Error:         `upstream status 502: {"error":{"message":"bad gateway"}}`,
	})
	s.RecordAttempt(FailoverAttempt{
		Iter:          1,
		Switches:      1,
		ChannelId:     8,
		ChannelName:   "backup",
		ApiType:       "Anthropic",
		OriginModel:   "gpt-4o",
		UpstreamModel: "claude-real",
		Status:        http.StatusOK,
		Decision:      "success",
	})

	chain := s.ChainJSON()
	if chain == "" {
		t.Fatal("chain json should not be empty")
	}
	var attempts []FailoverAttempt
	if err := json.Unmarshal([]byte(chain), &attempts); err != nil {
		t.Fatalf("unmarshal chain: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(attempts))
	}
	if attempts[0].ChannelName != "primary" || attempts[0].Decision != "switch_channel" {
		t.Fatalf("first attempt not preserved: %+v", attempts[0])
	}
	if !strings.Contains(attempts[0].Error, "bad gateway") || strings.Contains(attempts[0].Error, "upstream status") {
		t.Fatalf("attempt error should be cleaned, got %q", attempts[0].Error)
	}
	if attempts[1].Decision != "success" || attempts[1].Status != http.StatusOK {
		t.Fatalf("success attempt not preserved: %+v", attempts[1])
	}
}

func TestFailoverDecisionLabel(t *testing.T) {
	cases := map[FailoverDecision]string{
		DecisionRetrySameChannel: "retry_same_channel",
		DecisionSwitchChannel:    "switch_channel",
		DecisionFatal:            "fatal",
	}
	for decision, want := range cases {
		if got := failoverDecisionLabel(decision); got != want {
			t.Fatalf("decision label = %q, want %q", got, want)
		}
	}
}
