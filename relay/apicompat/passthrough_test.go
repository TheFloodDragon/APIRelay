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
