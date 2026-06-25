package relay

import (
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/model"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	// 纯内存 sqlite（共享缓存），每次清空相关表避免用例间污染
	if err := model.InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: "file::memory:?cache=shared"}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	model.DB.Exec("DELETE FROM channels")
	model.DB.Exec("DELETE FROM abilities")
}

func mustChannel(t *testing.T, name, models, group string, priority, weight int) *model.Channel {
	t.Helper()
	ch := &model.Channel{
		Name: name, Type: 1, Status: model.ChannelStatusEnabled,
		BaseURL: "http://x", Key: "k", Group: group,
		Models: models, Priority: priority, Weight: weight,
	}
	if err := model.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return ch
}

func TestSelectChannel_TieredDegradation(t *testing.T) {
	setupTestDB(t)
	// 高优先级渠道(priority=10) 与 低优先级渠道(priority=1)
	high := mustChannel(t, "high", "gpt-4o", "default", 10, 1)
	low := mustChannel(t, "low", "gpt-4o", "default", 1, 1)

	// 无排除：应选高优先级
	ch, err := SelectChannel("default", "gpt-4o", nil, 0)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if ch == nil || ch.Id != high.Id {
		t.Fatalf("expected high priority channel, got %v", ch)
	}

	// 排除高优先级：应降级到低优先级
	excluded := map[int]struct{}{high.Id: {}}
	ch, err = SelectChannel("default", "gpt-4o", excluded, 0)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if ch == nil || ch.Id != low.Id {
		t.Fatalf("expected degrade to low priority channel, got %v", ch)
	}
}

func TestSelectChannel_Wildcard(t *testing.T) {
	setupTestDB(t)
	// 通配渠道支持任意模型
	wild := mustChannel(t, "wild", "*", "default", 5, 1)

	ch, err := SelectChannel("default", "any-unknown-model", nil, 0)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if ch == nil || ch.Id != wild.Id {
		t.Fatalf("wildcard channel should serve unknown model, got %v", ch)
	}
}

func TestSelectChannel_CooldownSkipped(t *testing.T) {
	setupTestDB(t)
	a := mustChannel(t, "a", "gpt-4o", "default", 5, 1)
	b := mustChannel(t, "b", "gpt-4o", "default", 5, 1)

	// 让 a 处于冷却中
	model.SetChannelCooldown(a.Id, 9_000_000_000_000) // 远未来
	ch, err := SelectChannel("default", "gpt-4o", nil, 1_000_000_000_000)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if ch == nil || ch.Id != b.Id {
		t.Fatalf("cooled channel a should be skipped, got %v", ch)
	}
}
