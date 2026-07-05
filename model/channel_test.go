package model

import "testing"

func TestModelConfigList_DerivesFromLegacyModels(t *testing.T) {
	ch := &Channel{Models: "gpt-4o, claude-3 , "}
	cfgs := ch.ModelConfigList()
	if len(cfgs) != 2 {
		t.Fatalf("derived configs = %d, want 2", len(cfgs))
	}
	for _, m := range cfgs {
		if !m.Enabled {
			t.Errorf("derived model %q should be enabled", m.Name)
		}
	}
}

func TestModelConfigList_ParsesJSON(t *testing.T) {
	ch := &Channel{ModelConfigs: `[{"name":"a","enabled":true,"protocol":"anthropic","upstream":"a-real"},{"name":"b","enabled":false}]`}
	cfgs := ch.ModelConfigList()
	if len(cfgs) != 2 {
		t.Fatalf("configs = %d, want 2", len(cfgs))
	}
	if names := ch.EnabledModelNames(); len(names) != 1 || names[0] != "a" {
		t.Errorf("enabled names = %v, want [a]", names)
	}
}

func TestMappedModel_Priority(t *testing.T) {
	ch := &Channel{
		ModelConfigs: `[{"name":"gpt-4","enabled":true,"upstream":"gpt-4o"}]`,
		ModelMapping: `{"gpt-4":"legacy-model"}`,
	}
	// ModelConfigs.Upstream 优先于旧 ModelMapping
	if got := ch.MappedModel("gpt-4"); got != "gpt-4o" {
		t.Errorf("MappedModel = %q, want gpt-4o", got)
	}
	// 回退旧 ModelMapping
	ch2 := &Channel{
		ModelConfigs: `[{"name":"gpt-4","enabled":true}]`,
		ModelMapping: `{"gpt-4":"legacy-model"}`,
	}
	if got := ch2.MappedModel("gpt-4"); got != "legacy-model" {
		t.Errorf("MappedModel fallback = %q, want legacy-model", got)
	}
	// 无映射用原名
	if got := ch2.MappedModel("unknown"); got != "unknown" {
		t.Errorf("MappedModel default = %q, want unknown", got)
	}
}

func TestBackfillModels(t *testing.T) {
	ch := &Channel{ModelConfigs: `[{"name":"a","enabled":true},{"name":"b","enabled":false},{"name":"c","enabled":true}]`}
	ch.backfillModels()
	if ch.Models != "a,c" {
		t.Errorf("backfilled Models = %q, want a,c", ch.Models)
	}
}

func TestSafeHeaderOverrideMap(t *testing.T) {
	ch := &Channel{HeaderOverride: `{
		"Authorization":"Bearer bad",
		"x-api-key":"bad-key",
		"anthropic-version":"2024-01-01",
		"Content-Length":"123",
		"Host":"evil.example",
		"Connection":"close",
		"Transfer-Encoding":"chunked",
		"Content-Type":"text/plain",
		"X-Custom-Trace":"trace-1",
		" x-extra ":"kept"
	}`}

	safe := ch.SafeHeaderOverrideMap()
	for _, denied := range []string{
		"Authorization",
		"X-Api-Key",
		"Anthropic-Version",
		"Content-Length",
		"Host",
		"Connection",
		"Transfer-Encoding",
		"Content-Type",
	} {
		if _, ok := safe[denied]; ok {
			t.Fatalf("denied header %q should be filtered, got %v", denied, safe)
		}
	}
	if safe["X-Custom-Trace"] != "trace-1" {
		t.Errorf("custom header missing: %v", safe)
	}
	if safe["X-Extra"] != "kept" {
		t.Errorf("trimmed custom header missing: %v", safe)
	}
}
