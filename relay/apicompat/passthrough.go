package apicompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/apirelay/apirelay/constant"
)

// ============================================================================
// 同协议请求透传（B1）
//
// 当对外协议（EndpointType）与上游协议（APIType）一致时，请求体基于原始字节
// ir.Raw 仅改写顶层 "model" 字段整体透传，其余字节零改动，从而保留 tool_choice /
// metadata / thinking / cache_control 等 IR 未建模的字段，避免有损重建。
// ============================================================================

// errModelFieldNotFound 表示原始请求体中没有顶层 "model" 字符串字段（调用方应回退 IR 重建）。
var errModelFieldNotFound = errors.New("top-level model field not found")

// SameProtocol 判断对外协议与上游协议是否一致（可走零改写透传）。
func SameProtocol(ep constant.EndpointType, apiType constant.APIType) bool {
	switch ep {
	case constant.EndpointOpenAI:
		return apiType == constant.APITypeOpenAI
	case constant.EndpointAnthropic:
		return apiType == constant.APITypeAnthropic
	case constant.EndpointResponses:
		return apiType == constant.APITypeResponses
	default:
		return false
	}
}

// ReplaceTopLevelModel 仅替换 JSON 对象顶层 "model" 字段的值，其余字节保持不动。
//
// 通过 json.Decoder 的 Token 流定位顶层 "model" 值的字节区间，只重写该区间，
// 因此除 model 值外与原始输入逐字节一致（保序、保留未知字段与原始空白）。
// 嵌套对象/数组中的 "model" 键不会被误伤。若顶层不是对象、缺少 model 字段或
// model 值非字符串，返回 errModelFieldNotFound，调用方应回退 IR 重建。
func ReplaceTopLevelModel(raw []byte, newModel string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))

	openTok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	if delim, ok := openTok.(json.Delim); !ok || delim != '{' {
		return nil, errModelFieldNotFound
	}

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decode key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected non-string object key")
		}

		if key != "model" {
			if err := skipValue(dec); err != nil {
				return nil, err
			}
			continue
		}

		// 命中顶层 model：定位其字符串值的字节区间。
		// keyEnd 指向 key 字符串闭合引号之后（冒号与空白之前）。
		keyEnd := dec.InputOffset()
		valTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decode model value: %w", err)
		}
		if _, ok := valTok.(string); !ok {
			return nil, errModelFieldNotFound
		}
		valEnd := dec.InputOffset()
		// 冒号与空白不含引号，故 keyEnd 之后首个 '"' 即字符串值起始。
		valStart := bytes.IndexByte(raw[keyEnd:valEnd], '"')
		if valStart < 0 {
			return nil, errModelFieldNotFound
		}
		valStart += int(keyEnd)

		encoded, err := json.Marshal(newModel)
		if err != nil {
			return nil, fmt.Errorf("encode model: %w", err)
		}
		out := make([]byte, 0, len(raw)-(int(valEnd)-valStart)+len(encoded))
		out = append(out, raw[:valStart]...)
		out = append(out, encoded...)
		out = append(out, raw[valEnd:]...)
		return out, nil
	}
	return nil, errModelFieldNotFound
}

// skipValue 消费 decoder 的下一个完整值（标量或嵌套结构），保持解析位置平衡。
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("decode value: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || (delim != '{' && delim != '[') {
		return nil // 标量值
	}
	inner := 1
	for inner > 0 {
		t, err := dec.Token()
		if err != nil {
			return fmt.Errorf("decode nested: %w", err)
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				inner++
			case '}', ']':
				inner--
			}
		}
	}
	return nil
}
