package relay

import (
	"net/http"
	"testing"

	"github.com/apirelay/apirelay/model"
)

func TestFailoverState_FatalOnNonRetryable(t *testing.T) {
	s := NewFailoverState(60)
	d := s.OnFailure(1, http.StatusBadRequest, false, "bad request")
	if d != DecisionFatal {
		t.Fatalf("expected fatal, got %d", d)
	}
}

func TestFailoverState_SameChannelThenSwitch(t *testing.T) {
	s := NewFailoverState(60)
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
	s := NewFailoverState(60)
	// 502 可重试但非"瞬时"类别，应直接切换渠道而非同渠道重试
	d := s.OnFailure(3, http.StatusBadGateway, true, "bad gateway")
	if d != DecisionSwitchChannel {
		t.Fatalf("expected switch for 502, got %d", d)
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
