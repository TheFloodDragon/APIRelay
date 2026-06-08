package scheduler

import (
	"testing"

	"github.com/TheFloodDragon/APIRelay/internal/model"
)

func TestSelectWeightedWithSingleChannel(t *testing.T) {
	channels := []model.Channel{{ID: 7, Weight: 10}}
	selected := selectWeighted(channels)
	if selected == nil || selected.ID != 7 {
		t.Fatalf("expected channel 7, got %#v", selected)
	}
}

func TestSelectWeightedFallsBackWhenWeightsAreNotPositive(t *testing.T) {
	channels := []model.Channel{
		{ID: 1, Weight: 0},
		{ID: 2, Weight: -3},
	}
	selected := selectWeighted(channels)
	if selected == nil || selected.ID != 1 {
		t.Fatalf("expected first channel fallback, got %#v", selected)
	}
}

func TestRoundRobinSelectsChannelsInOrderPerModel(t *testing.T) {
	s := NewScheduler(nil, "round_robin")
	channels := []model.Channel{{ID: 1}, {ID: 2}, {ID: 3}}

	want := []uint{1, 2, 3, 1, 2}
	for i, id := range want {
		selected := s.selectRoundRobin("gpt-test", channels)
		if selected == nil || selected.ID != id {
			t.Fatalf("selection %d: expected channel %d, got %#v", i, id, selected)
		}
	}
}

func TestRoundRobinCountersAreScopedByModel(t *testing.T) {
	s := NewScheduler(nil, "round_robin")
	channels := []model.Channel{{ID: 1}, {ID: 2}}

	if selected := s.selectRoundRobin("model-a", channels); selected == nil || selected.ID != 1 {
		t.Fatalf("model-a first selection expected channel 1, got %#v", selected)
	}
	if selected := s.selectRoundRobin("model-a", channels); selected == nil || selected.ID != 2 {
		t.Fatalf("model-a second selection expected channel 2, got %#v", selected)
	}
	if selected := s.selectRoundRobin("model-b", channels); selected == nil || selected.ID != 1 {
		t.Fatalf("model-b first selection expected independent channel 1, got %#v", selected)
	}
}

func TestOrderChannelsMovesSelectedChannelToFront(t *testing.T) {
	s := NewScheduler(nil, "round_robin")
	channels := []model.Channel{{ID: 1}, {ID: 2}, {ID: 3}}

	first := s.orderChannels("gpt-test", channels)
	if len(first) != 3 || first[0].ID != 1 {
		t.Fatalf("first ordered list should start with channel 1, got %#v", first)
	}

	second := s.orderChannels("gpt-test", channels)
	if len(second) != 3 || second[0].ID != 2 || second[1].ID != 1 || second[2].ID != 3 {
		t.Fatalf("second ordered list should move channel 2 to front and preserve others, got %#v", second)
	}
}
