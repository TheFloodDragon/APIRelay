package apicompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// RewriteRawSSEModel 将同协议 Raw SSE 的响应模型名回写为客户端请求的显示模型名。
// 非 data 行、终止标记、无模型字段或无效 JSON 均原样返回；只有模型承载事件会被重序列化。
func RewriteRawSSEModel(line string, ep constant.EndpointType, displayModel string) string {
	if displayModel == "" || !strings.HasPrefix(line, "data:") {
		return line
	}
	payloadStart := len("data:")
	for payloadStart < len(line) && (line[payloadStart] == ' ' || line[payloadStart] == '\t') {
		payloadStart++
	}
	payload := line[payloadStart:]
	if payload == "" || payload == "[DONE]" {
		return line
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return line
	}
	changed := false
	switch ep {
	case constant.EndpointAnthropic:
		if message, ok := root["message"].(map[string]any); ok {
			if _, exists := message["model"]; exists {
				message["model"] = displayModel
				changed = true
			}
		}
	case constant.EndpointResponses:
		if response, ok := root["response"].(map[string]any); ok {
			if _, exists := response["model"]; exists {
				response["model"] = displayModel
				changed = true
			}
		}
	default:
		if _, exists := root["model"]; exists {
			root["model"] = displayModel
			changed = true
		}
	}
	if !changed {
		return line
	}
	rewritten, err := json.Marshal(root)
	if err != nil {
		return line
	}
	return line[:payloadStart] + string(rewritten)
}

// ============================================================================
// Body 复写（CC Switch 语义）
//
// 在协议转换之后、发往上游之前，把渠道配置的 patch 深合并进最终请求体：
//   - object 与 object 递归合并（保留目标已有键）；
//   - 其余情况（数组、标量、null、类型不一致）整体替换目标值；
//   - 数组不逐项合并，整体替换；null 是普通值覆盖，不表示删除；
//   - 顶层保护字段（stream）由 model.SafeBodyOverride 预先剔除，这里再兜底跳过。
// ============================================================================

// bodyOverrideProtectedTopLevel 顶层保护字段（与 model 层保持一致的兜底）。
var bodyOverrideProtectedTopLevel = map[string]struct{}{
	"stream": {},
}

// ApplyBodyOverride 把 patch 深合并进原始请求体 raw，返回合并后的 JSON 字节。
//
// raw 必须是 JSON 对象；patch 为空时原样返回 raw（不做解析开销）。
// 若 raw 解析失败或不是对象，返回错误，调用方应保留原始 raw 不做改写。
func ApplyBodyOverride(raw []byte, patch map[string]any) ([]byte, error) {
	if len(patch) == 0 {
		return raw, nil
	}
	var target any
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	targetMap, ok := target.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("request body is not a JSON object")
	}
	mergeJSONObject(targetMap, patch, true)
	out, err := json.Marshal(targetMap)
	if err != nil {
		return nil, fmt.Errorf("encode merged body: %w", err)
	}
	return out, nil
}

// mergeJSONObject 将 patch 深合并进 target（就地修改 target）。
// topLevel 为 true 时跳过受保护顶层字段。
func mergeJSONObject(target, patch map[string]any, topLevel bool) {
	for key, patchVal := range patch {
		if topLevel {
			if _, protected := bodyOverrideProtectedTopLevel[key]; protected {
				continue
			}
		}
		existing, ok := target[key]
		if !ok {
			target[key] = patchVal
			continue
		}
		existingObj, eOK := existing.(map[string]any)
		patchObj, pOK := patchVal.(map[string]any)
		if eOK && pOK {
			mergeJSONObject(existingObj, patchObj, false)
			continue
		}
		// 非 object-object：整体替换（含数组、标量、null、类型不一致）。
		target[key] = patchVal
	}
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
