package relay

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

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
