package model

import "testing"

func TestBuildKeyOverlayVirtualKeyReusesChannel(t *testing.T) {
	ch := &Channel{Id: 1, Key: "sk-channel", BaseURL: "https://api.example.com"}
	// 虚拟 Key（Id<=0）直接复用原渠道对象。
	got := BuildKeyOverlay(ch, &ChannelKey{Id: 0, Key: "ignored"})
	if got != ch {
		t.Fatal("virtual key should reuse the original channel pointer")
	}
	// nil key 同样复用。
	if BuildKeyOverlay(ch, nil) != ch {
		t.Fatal("nil key should reuse the original channel pointer")
	}
}

func TestBuildKeyOverlayReplacesKeyOnly(t *testing.T) {
	ch := &Channel{Id: 1, Key: "sk-channel", ModelConfigs: `[{"name":"gpt-4o","enabled":true}]`}
	key := &ChannelKey{Id: 5, Key: "sk-key-5"}
	overlay := BuildKeyOverlay(ch, key)

	if overlay == ch {
		t.Fatal("real key must produce a copy, not the original")
	}
	if overlay.Key != "sk-key-5" {
		t.Fatalf("overlay key = %q, want sk-key-5", overlay.Key)
	}
	// 原渠道不被修改。
	if ch.Key != "sk-channel" {
		t.Fatalf("original channel key mutated: %q", ch.Key)
	}
	// 无 Key 级模型覆盖时 ModelConfigs 原样保留。
	if overlay.ModelConfigs != ch.ModelConfigs {
		t.Fatalf("model configs changed unexpectedly: %q", overlay.ModelConfigs)
	}
}

func TestBuildKeyOverlayMergesModelConfigs(t *testing.T) {
	ch := &Channel{
		Id:  1,
		Key: "sk-channel",
		ModelConfigs: `[
			{"name":"gpt-4o","enabled":true,"upstream":"gpt-4o-base","protocol":"openai","input":1.0,"output":2.0},
			{"name":"claude","enabled":true,"protocol":"anthropic"}
		]`,
	}
	// Key 级覆盖 gpt-4o 的 upstream/价格；引用一个不存在的模型（应被忽略，不改变模型集合）。
	key := &ChannelKey{
		Id:  7,
		Key: "sk-key-7",
		ModelConfigs: `[
			{"name":"gpt-4o","upstream":"gpt-4o-key7","input":5.0},
			{"name":"ghost","upstream":"should-be-ignored"}
		]`,
	}
	overlay := BuildKeyOverlay(ch, key)

	// 用合并后的配置解析：gpt-4o 走 Key 级 upstream 与 input，其余字段保留渠道级。
	m, ok := overlay.ModelConfig("gpt-4o")
	if !ok {
		t.Fatal("gpt-4o should exist in overlay")
	}
	if m.Upstream != "gpt-4o-key7" {
		t.Fatalf("upstream = %q, want gpt-4o-key7 (key-level override)", m.Upstream)
	}
	if m.Input != 5.0 {
		t.Fatalf("input = %v, want 5.0 (key-level override)", m.Input)
	}
	if m.Output != 2.0 {
		t.Fatalf("output = %v, want 2.0 (inherited from channel)", m.Output)
	}
	if m.Protocol != "openai" {
		t.Fatalf("protocol = %q, want openai (inherited, key didn't override)", m.Protocol)
	}
	// claude 未被 Key 覆盖，保持渠道级配置。
	if cm, _ := overlay.ModelConfig("claude"); cm.Protocol != "anthropic" {
		t.Fatalf("claude protocol = %q, want anthropic", cm.Protocol)
	}
	// 不存在的 "ghost" 不应被加入。
	if _, ok := overlay.ModelConfig("ghost"); ok {
		t.Fatal("ghost model should not be added to overlay (model set must not change)")
	}
	// 原渠道 ModelConfigs 不被修改。
	if got, _ := ch.ModelConfig("gpt-4o"); got.Upstream != "gpt-4o-base" {
		t.Fatalf("original channel model mutated: upstream=%q", got.Upstream)
	}
}
