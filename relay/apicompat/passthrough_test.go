package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/apirelay/apirelay/constant"
)

func TestSameProtocol(t *testing.T) {
	cases := []struct {
		ep   constant.EndpointType
		at   constant.APIType
		want bool
	}{
		{constant.EndpointOpenAI, constant.APITypeOpenAI, true},
		{constant.EndpointOpenAI, constant.APITypeAnthropic, false},
		{constant.EndpointAnthropic, constant.APITypeAnthropic, true},
		{constant.EndpointAnthropic, constant.APITypeResponses, false},
		{constant.EndpointResponses, constant.APITypeResponses, true},
		{constant.EndpointResponses, constant.APITypeOpenAI, false},
	}
	for _, c := range cases {
		if got := SameProtocol(c.ep, c.at); got != c.want {
			t.Errorf("SameProtocol(%v,%v) = %v, want %v", c.ep, c.at, got, c.want)
		}
	}
}

// TestReplaceTopLevelModel_ByteExact 验证除 model 值外逐字节保持不变，未知字段保留。
func TestReplaceTopLevelModel_ByteExact(t *testing.T) {
	raw := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto","metadata":{"model":"nested-should-not-change"},"stream":true}`)
	out, err := ReplaceTopLevelModel(raw, "real-model")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	want := `{"model":"real-model","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto","metadata":{"model":"nested-should-not-change"},"stream":true}`
	if string(out) != want {
		t.Fatalf("byte-exact replace failed:\n got: %s\nwant: %s", out, want)
	}
}

// TestReplaceTopLevelModel_NestedNotTouched 嵌套对象/数组内的 model 不应被改。
func TestReplaceTopLevelModel_NestedNotTouched(t *testing.T) {
	raw := []byte(`{"tools":[{"model":"x"}],"model":"target","nested":{"a":{"model":"y"}}}`)
	out, err := ReplaceTopLevelModel(raw, "NEW")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("result not valid json: %v", err)
	}
	var top string
	_ = json.Unmarshal(m["model"], &top)
	if top != "NEW" {
		t.Errorf("top-level model = %q, want NEW", top)
	}
	// 嵌套值保持原样
	if string(m["tools"]) != `[{"model":"x"}]` {
		t.Errorf("nested tools mutated: %s", m["tools"])
	}
	if string(m["nested"]) != `{"a":{"model":"y"}}` {
		t.Errorf("nested object mutated: %s", m["nested"])
	}
}

// TestReplaceTopLevelModel_Escaping model 值含需转义字符时正确编码。
func TestReplaceTopLevelModel_Escaping(t *testing.T) {
	raw := []byte(`{"model":"old","x":1}`)
	out, err := ReplaceTopLevelModel(raw, `a"b\c`)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	var m struct {
		Model string `json:"model"`
		X     int    `json:"x"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("result not valid json: %v (%s)", err, out)
	}
	if m.Model != `a"b\c` || m.X != 1 {
		t.Errorf("decoded = %#v", m)
	}
}

// TestReplaceTopLevelModel_Whitespacepreserved 保留原始空白与字段顺序。
func TestReplaceTopLevelModel_WhitespacePreserved(t *testing.T) {
	raw := []byte("{\n  \"a\": 1,\n  \"model\": \"old\",\n  \"b\": 2\n}")
	out, err := ReplaceTopLevelModel(raw, "new")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	want := "{\n  \"a\": 1,\n  \"model\": \"new\",\n  \"b\": 2\n}"
	if string(out) != want {
		t.Fatalf("whitespace not preserved:\n got: %q\nwant: %q", out, want)
	}
}

// TestReplaceTopLevelModel_Errors 缺字段/非对象/非字符串值应报错以便回退 IR。
func TestReplaceTopLevelModel_Errors(t *testing.T) {
	cases := []string{
		`{"messages":[]}`,   // 缺 model
		`[1,2,3]`,           // 非对象
		`{"model":123}`,     // model 非字符串
		`{"model":{"a":1}}`, // model 是对象
	}
	for _, in := range cases {
		if _, err := ReplaceTopLevelModel([]byte(in), "x"); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

// TestApplyBodyOverride_EmptyPatch 空补丁原样返回。
func TestApplyBodyOverride_EmptyPatch(t *testing.T) {
	raw := []byte(`{"model":"m","stream":true}`)
	out, err := ApplyBodyOverride(raw, nil)
	if err != nil {
		t.Fatalf("empty patch: %v", err)
	}
	if string(out) != string(raw) {
		t.Fatalf("empty patch should return raw unchanged: %s", out)
	}
}

// TestApplyBodyOverride_DeepMerge object 递归合并、数组整体替换、标量覆盖、新增字段。
func TestApplyBodyOverride_DeepMerge(t *testing.T) {
	raw := []byte(`{"model":"before","metadata":{"keep":true,"temperature":1},"messages":[{"role":"user","content":"hi"}]}`)
	patch := map[string]any{
		"model":    "after",
		"metadata": map[string]any{"temperature": 0.2, "top_p": 0.9},
		"messages": []any{},
		"new_key":  "added",
	}
	out, err := ApplyBodyOverride(raw, patch)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("result not valid json: %v", err)
	}
	if got["model"] != "after" {
		t.Errorf("model = %v, want after", got["model"])
	}
	if got["new_key"] != "added" {
		t.Errorf("new_key = %v, want added", got["new_key"])
	}
	meta, ok := got["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata not object: %v", got["metadata"])
	}
	// 递归合并：保留 keep，覆盖 temperature，新增 top_p
	if meta["keep"] != true {
		t.Errorf("metadata.keep should be preserved, got %v", meta["keep"])
	}
	if meta["temperature"] != 0.2 {
		t.Errorf("metadata.temperature = %v, want 0.2", meta["temperature"])
	}
	if meta["top_p"] != 0.9 {
		t.Errorf("metadata.top_p = %v, want 0.9", meta["top_p"])
	}
	// 数组整体替换为空
	arr, ok := got["messages"].([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("messages should be replaced with [], got %v", got["messages"])
	}
}

// TestApplyBodyOverride_ArrayReplacesObject 类型不一致时整体替换。
func TestApplyBodyOverride_TypeMismatchReplaces(t *testing.T) {
	raw := []byte(`{"a":{"nested":1},"b":[1,2]}`)
	patch := map[string]any{
		"a": []any{"now-array"},
		"b": map[string]any{"now": "object"},
	}
	out, err := ApplyBodyOverride(raw, patch)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if _, ok := got["a"].([]any); !ok {
		t.Errorf("a should be replaced by array, got %T", got["a"])
	}
	if _, ok := got["b"].(map[string]any); !ok {
		t.Errorf("b should be replaced by object, got %T", got["b"])
	}
}

// TestApplyBodyOverride_NullOverwrites null 是普通值覆盖，不表示删除。
func TestApplyBodyOverride_NullOverwrites(t *testing.T) {
	raw := []byte(`{"metadata":{"foo":"bar"}}`)
	patch := map[string]any{"metadata": nil}
	out, err := ApplyBodyOverride(raw, patch)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if v, ok := got["metadata"]; !ok || v != nil {
		t.Errorf("metadata should be null, got %v (present=%v)", v, ok)
	}
}

// TestApplyBodyOverride_ProtectedStream 顶层 stream 不可被合并覆盖（兜底）。
func TestApplyBodyOverride_ProtectedStream(t *testing.T) {
	raw := []byte(`{"stream":false,"nested":{"stream":false}}`)
	patch := map[string]any{
		"stream": true,
		"nested": map[string]any{"stream": true},
	}
	out, err := ApplyBodyOverride(raw, patch)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)
	if got["stream"] != false {
		t.Errorf("top-level stream must stay false, got %v", got["stream"])
	}
	// 嵌套 stream 不受保护，可被覆盖
	nested := got["nested"].(map[string]any)
	if nested["stream"] != true {
		t.Errorf("nested stream should be overwritten to true, got %v", nested["stream"])
	}
}

// TestApplyBodyOverride_NonObjectBody 非对象请求体报错。
func TestApplyBodyOverride_NonObjectBody(t *testing.T) {
	if _, err := ApplyBodyOverride([]byte(`[1,2,3]`), map[string]any{"a": 1}); err == nil {
		t.Error("expected error for non-object body")
	}
}
