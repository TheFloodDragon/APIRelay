package dto

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestStopSequences_Unmarshal 验证 stop 字段兼容字符串与数组两种形态。
func TestStopSequences_Unmarshal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want StopSequences
	}{
		{"single_string", `"STOP"`, StopSequences{"STOP"}},
		{"array", `["a","b"]`, StopSequences{"a", "b"}},
		{"empty_array", `[]`, StopSequences{}},
		{"null", `null`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s StopSequences
			if err := json.Unmarshal([]byte(c.in), &s); err != nil {
				t.Fatalf("unmarshal %s: %v", c.in, err)
			}
			if !reflect.DeepEqual(s, c.want) {
				t.Fatalf("got %#v, want %#v", s, c.want)
			}
		})
	}
}

// TestStopSequences_UnmarshalInRequest 验证请求体中字符串形式的 stop 不再报错。
func TestStopSequences_UnmarshalInRequest(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[],"stop":"\n\n"}`
	var req OpenAIChatRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("string stop should parse: %v", err)
	}
	if len(req.Stop) != 1 || req.Stop[0] != "\n\n" {
		t.Fatalf("stop = %#v, want [\\n\\n]", req.Stop)
	}

	bodyArr := `{"model":"gpt-4o","messages":[],"stop":["a","b"]}`
	var reqArr OpenAIChatRequest
	if err := json.Unmarshal([]byte(bodyArr), &reqArr); err != nil {
		t.Fatalf("array stop should parse: %v", err)
	}
	if !reflect.DeepEqual([]string(reqArr.Stop), []string{"a", "b"}) {
		t.Fatalf("stop = %#v", reqArr.Stop)
	}
}

// TestStopSequences_Marshal 验证序列化按数组输出，空值省略。
func TestStopSequences_Marshal(t *testing.T) {
	// 非空 → 数组
	req := OpenAIChatRequest{Model: "gpt-4o", Stop: StopSequences{"x", "y"}}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round map[string]json.RawMessage
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal round: %v", err)
	}
	if string(round["stop"]) != `["x","y"]` {
		t.Fatalf("marshaled stop = %s, want [\"x\",\"y\"]", round["stop"])
	}

	// 空 → 省略（omitempty）
	empty := OpenAIChatRequest{Model: "gpt-4o"}
	b2, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	var round2 map[string]json.RawMessage
	if err := json.Unmarshal(b2, &round2); err != nil {
		t.Fatalf("unmarshal round2: %v", err)
	}
	if _, ok := round2["stop"]; ok {
		t.Fatalf("empty stop should be omitted, got %s", b2)
	}
}
