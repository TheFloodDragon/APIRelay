package relay

import (
	"strings"
	"testing"
)

func TestNoChannelError_GroupMismatch(t *testing.T) {
	setupTestDB(t)
	// 在 vip 分组配置 claude-opus，但请求用 default 分组
	mustChannel(t, "vip-provider", "claude-opus-4-8", "vip", 0, 1)

	msg := noChannelError("default", "claude-opus-4-8")
	if !strings.Contains(msg, "claude-opus-4-8") {
		t.Errorf("missing model name: %q", msg)
	}
	if !strings.Contains(msg, "vip-provider") {
		t.Errorf("should mention provider in other group: %q", msg)
	}
	if !strings.Contains(msg, "分组") {
		t.Errorf("should explain group mismatch: %q", msg)
	}
}

func TestNoChannelError_ListsAvailableModels(t *testing.T) {
	setupTestDB(t)
	mustChannel(t, "p1", "gpt-4o,gpt-4o-mini", "default", 0, 1)

	msg := noChannelError("default", "nonexistent-model")
	if !strings.Contains(msg, "nonexistent-model") {
		t.Errorf("missing requested model: %q", msg)
	}
	// 应列出当前分组可用模型
	if !strings.Contains(msg, "gpt-4o") {
		t.Errorf("should list available models: %q", msg)
	}
}

func TestNoChannelError_EmptyGroup(t *testing.T) {
	setupTestDB(t)
	msg := noChannelError("default", "any-model")
	if !strings.Contains(msg, "暂无任何已启用的模型") {
		t.Errorf("should say group has no models: %q", msg)
	}
}
