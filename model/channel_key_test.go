package model

import (
	"testing"
)

// resetChannelKeyTables 清空渠道与 Key 表，保证用例间互不干扰。
func resetChannelKeyTables(t *testing.T) {
	t.Helper()
	DB.Exec("DELETE FROM channel_keys")
	DB.Exec("DELETE FROM channels")
}

func TestCreateChannelKeyAutoIndexAndDefaults(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "multi-key", Status: ChannelStatusEnabled, Key: "sk-legacy"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}

	// KeyIndex<0 时自动追加到末尾。
	first := &ChannelKey{ChannelId: ch.Id, KeyIndex: -1, Key: "sk-a"}
	if err := CreateChannelKey(first); err != nil {
		t.Fatal(err)
	}
	if first.KeyIndex != 0 {
		t.Fatalf("first KeyIndex = %d, want 0", first.KeyIndex)
	}
	if first.Status != ChannelKeyStatusEnabled {
		t.Fatalf("first Status = %d, want enabled", first.Status)
	}
	if !first.Available {
		t.Fatal("first key should default Available=true")
	}

	second := &ChannelKey{ChannelId: ch.Id, KeyIndex: -1, Key: "sk-b"}
	if err := CreateChannelKey(second); err != nil {
		t.Fatal(err)
	}
	if second.KeyIndex != 1 {
		t.Fatalf("second KeyIndex = %d, want 1", second.KeyIndex)
	}
}

func TestListChannelKeysOrdersByIndex(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "ordered", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	// 乱序插入，验证按 KeyIndex 升序返回。
	for _, idx := range []int{2, 0, 1} {
		k := &ChannelKey{ChannelId: ch.Id, KeyIndex: idx, Key: "sk", Status: ChannelKeyStatusEnabled}
		if err := DB.Create(k).Error; err != nil {
			t.Fatal(err)
		}
	}
	list, err := ListChannelKeys(ch.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("keys = %d, want 3", len(list))
	}
	for i, k := range list {
		if k.KeyIndex != i {
			t.Fatalf("list[%d].KeyIndex = %d, want %d", i, k.KeyIndex, i)
		}
	}
}

func TestLoadEnabledChannelKeysFallbackVirtual(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	// 无子表记录：从 Channel.Key 派生虚拟单 Key。
	ch := &Channel{Name: "legacy", Status: ChannelStatusEnabled, Key: "sk-legacy"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	keys, err := LoadEnabledChannelKeys(ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("virtual keys = %d, want 1", len(keys))
	}
	if !keys[0].IsVirtual() {
		t.Fatal("fallback key should be virtual (Id<=0)")
	}
	if keys[0].Key != "sk-legacy" {
		t.Fatalf("virtual key = %q, want sk-legacy", keys[0].Key)
	}
}

func TestLoadEnabledChannelKeysNoKeyReturnsEmpty(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	// 无子表且渠道无凭据：返回空（无可用 Key）。
	ch := &Channel{Name: "no-key", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	keys, err := LoadEnabledChannelKeys(ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("keys = %d, want 0", len(keys))
	}
}

func TestLoadEnabledChannelKeysPrefersSubtableAndFiltersDisabled(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	// 有子表记录时忽略 Channel.Key，且过滤管理员禁用的 Key。
	ch := &Channel{Name: "sub", Status: ChannelStatusEnabled, Key: "sk-should-be-ignored"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	enabled := &ChannelKey{ChannelId: ch.Id, KeyIndex: 0, Key: "sk-enabled", Status: ChannelKeyStatusEnabled}
	disabled := &ChannelKey{ChannelId: ch.Id, KeyIndex: 1, Key: "sk-disabled", Status: ChannelKeyStatusDisabled}
	if err := DB.Create(enabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(disabled).Error; err != nil {
		t.Fatal(err)
	}
	keys, err := LoadEnabledChannelKeys(ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("enabled keys = %d, want 1", len(keys))
	}
	if keys[0].Key != "sk-enabled" || keys[0].IsVirtual() {
		t.Fatalf("unexpected key: %+v", keys[0])
	}
}

func TestBackfillChannelKeysIdempotent(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	withKey := &Channel{Name: "with-key", Status: ChannelStatusEnabled, Key: "sk-1"}
	noKey := &Channel{Name: "no-key", Status: ChannelStatusEnabled}
	if err := DB.Create(withKey).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(noKey).Error; err != nil {
		t.Fatal(err)
	}

	// 首次回填：仅为有凭据的渠道建立一条 Key。
	if err := BackfillChannelKeys(); err != nil {
		t.Fatal(err)
	}
	n, err := CountChannelKeys(withKey.Id)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("backfilled keys = %d, want 1", n)
	}
	if n2, _ := CountChannelKeys(noKey.Id); n2 != 0 {
		t.Fatalf("no-key channel should not be backfilled, got %d", n2)
	}

	// 再次回填：幂等，不重复插入。
	if err := BackfillChannelKeys(); err != nil {
		t.Fatal(err)
	}
	if n, _ := CountChannelKeys(withKey.Id); n != 1 {
		t.Fatalf("second backfill created duplicates: %d", n)
	}
}

func TestUpsertChannelKeyHealthDropsOlderSnapshot(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "health", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	k := &ChannelKey{ChannelId: ch.Id, KeyIndex: 0, Key: "sk", Status: ChannelKeyStatusEnabled, Available: true, PersistVersion: 2}
	if err := DB.Create(k).Error; err != nil {
		t.Fatal(err)
	}

	// 旧版本快照应被忽略。
	stale := &ChannelKey{Id: k.Id, Available: false, ConsecutiveFailures: 9, DisabledReason: string(DisableReasonAuthFailed), PersistVersion: 1}
	if err := UpsertChannelKeyHealth(stale); err != nil {
		t.Fatal(err)
	}
	got, err := GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.ConsecutiveFailures != 0 {
		t.Fatalf("stale snapshot overwrote state: %+v", got)
	}

	// 新版本快照应写入。
	fresh := &ChannelKey{Id: k.Id, Available: false, ConsecutiveFailures: 3, DisabledReason: string(DisableReasonQuotaExhausted), PersistVersion: 5}
	if err := UpsertChannelKeyHealth(fresh); err != nil {
		t.Fatal(err)
	}
	got, err = GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Available || got.ConsecutiveFailures != 3 || got.DisabledReason != string(DisableReasonQuotaExhausted) {
		t.Fatalf("fresh snapshot not applied: %+v", got)
	}
}

func TestResetChannelKeyClearsRuntimeState(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "reset", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	k := &ChannelKey{
		ChannelId: ch.Id, KeyIndex: 0, Key: "sk", Status: ChannelKeyStatusEnabled,
		Available: false, CooldownUntil: nowMilli() + 100000, ConsecutiveFailures: 5,
		DisabledReason: string(DisableReasonAuthFailed), LastError: "boom", PersistVersion: 3,
	}
	if err := DB.Create(k).Error; err != nil {
		t.Fatal(err)
	}
	if err := ResetChannelKey(k.Id); err != nil {
		t.Fatal(err)
	}
	got, err := GetChannelKeyByID(k.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.CooldownUntil != 0 || got.ConsecutiveFailures != 0 || got.DisabledReason != "" || got.LastError != "" {
		t.Fatalf("key not fully reset: %+v", got)
	}
	if got.PersistVersion != 4 {
		t.Fatalf("persist version = %d, want 4 (advanced)", got.PersistVersion)
	}
}

func TestReorderChannelKeys(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "reorder", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	var ids []int
	for i := 0; i < 3; i++ {
		k := &ChannelKey{ChannelId: ch.Id, KeyIndex: i, Key: "sk", Status: ChannelKeyStatusEnabled}
		if err := DB.Create(k).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, k.Id)
	}
	// 反转顺序。
	reversed := []int{ids[2], ids[1], ids[0]}
	if err := ReorderChannelKeys(ch.Id, reversed); err != nil {
		t.Fatal(err)
	}
	list, err := ListChannelKeys(ch.Id)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Id != ids[2] || list[2].Id != ids[0] {
		t.Fatalf("reorder failed: got order %d,%d,%d", list[0].Id, list[1].Id, list[2].Id)
	}
}

func TestDeleteChannelCascadesKeys(t *testing.T) {
	setupTestDB(t)
	resetChannelKeyTables(t)

	ch := &Channel{Name: "cascade", Status: ChannelStatusEnabled}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		k := &ChannelKey{ChannelId: ch.Id, KeyIndex: i, Key: "sk", Status: ChannelKeyStatusEnabled}
		if err := DB.Create(k).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteChannel(ch.Id); err != nil {
		t.Fatal(err)
	}
	if n, _ := CountChannelKeys(ch.Id); n != 0 {
		t.Fatalf("keys not cascaded on channel delete: %d remain", n)
	}
}

func TestApplySubmittedKeyChannelKey(t *testing.T) {
	// 留空沿用 existing。
	existing := &ChannelKey{Key: "old"}
	in := &ChannelKey{KeyInput: "  "}
	in.ApplySubmittedKey(existing)
	if in.Key != "old" {
		t.Fatalf("blank input should reuse existing, got %q", in.Key)
	}
	// 非空覆盖。
	in2 := &ChannelKey{KeyInput: "new"}
	in2.ApplySubmittedKey(existing)
	if in2.Key != "new" {
		t.Fatalf("non-blank input should override, got %q", in2.Key)
	}
	// 新建（existing=nil）留空为未配置。
	in3 := &ChannelKey{KeyInput: ""}
	in3.ApplySubmittedKey(nil)
	if in3.Key != "" {
		t.Fatalf("new key blank should stay empty, got %q", in3.Key)
	}
}

func TestChannelKeyModelConfigList(t *testing.T) {
	// 空配置返回 nil（继承渠道级）。
	empty := &ChannelKey{}
	if empty.ModelConfigList() != nil {
		t.Fatal("empty model configs should return nil")
	}
	// 合法 JSON 解析。
	k := &ChannelKey{ModelConfigs: `[{"name":"gpt-4o","enabled":true,"upstream":"gpt-4o-2024","protocol":"openai"}]`}
	cfgs := k.ModelConfigList()
	if len(cfgs) != 1 || cfgs[0].Upstream != "gpt-4o-2024" {
		t.Fatalf("parsed configs = %+v", cfgs)
	}
	// 非法 JSON 返回 nil。
	bad := &ChannelKey{ModelConfigs: `{not json`}
	if bad.ModelConfigList() != nil {
		t.Fatal("invalid json should return nil")
	}
}
