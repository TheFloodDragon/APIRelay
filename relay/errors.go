package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// errEmptyUpstreamResponse 表示上游返回了空响应（常见于 base_url/模型/密钥配置错误）。
var errEmptyUpstreamResponse = errors.New("empty upstream response")

// extractUpstreamErrorMessage 从上游错误响应体中提取人类可读的错误信息。
// 兼容 OpenAI / Anthropic / 通用 {error:{message}} / {message} / {error:"..."} 等结构；
// 解析失败时回退为截断后的原始文本。
func extractUpstreamErrorMessage(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}

	// 尝试结构化解析
	var probe struct {
		Error json.RawMessage `json:"error"`
		// 顶层 message（部分网关直接返回）
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(body, &probe); err == nil {
		if msg := messageFromErrorField(probe.Error); msg != "" {
			return msg
		}
		if probe.Message != "" {
			return probe.Message
		}
		if probe.Detail != "" {
			return probe.Detail
		}
	}

	// 回退：截断原始文本，去除换行
	return truncateMessage(strings.ReplaceAll(s, "\n", " "), 300)
}

// messageFromErrorField 解析 error 字段，可能是对象 {message,type,code} 或字符串。
func messageFromErrorField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 对象形式
	var obj struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Message != "" {
		return obj.Message
	}
	// 字符串形式
	var str string
	if err := json.Unmarshal(raw, &str); err == nil && str != "" {
		return str
	}
	return ""
}

// friendlyUpstreamError 为单渠道致命错误生成面向客户端的友好提示。
func friendlyUpstreamError(info *RelayInfo, status int, err error) string {
	if errors.Is(err, errEmptyUpstreamResponse) {
		return emptyResponseHint(info)
	}
	detail := extractUpstreamErrorMessage([]byte(stripInternalPrefix(err.Error())))
	base := statusHint(status)
	if detail != "" {
		return fmt.Sprintf("%s（上游返回：%s）", base, truncateMessage(detail, 300))
	}
	return base
}

// friendlyExhaustedError 为「所有渠道均失败」生成友好提示。
func friendlyExhaustedError(info *RelayInfo, status int, lastErr string) string {
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	model := info.OriginModel
	if strings.Contains(lastErr, errEmptyUpstreamResponse.Error()) {
		return emptyResponseHint(info)
	}
	detail := extractUpstreamErrorMessage([]byte(stripInternalPrefix(lastErr)))
	base := fmt.Sprintf("模型 %q 的所有可用渠道均请求失败（%s）", model, statusHint(status))
	if detail != "" {
		return fmt.Sprintf("%s。最后一次错误：%s", base, truncateMessage(detail, 300))
	}
	return base + "，请稍后重试或检查供应商配置。"
}

// emptyResponseHint 针对空响应给出可操作的排障提示。
func emptyResponseHint(info *RelayInfo) string {
	return fmt.Sprintf(
		"上游对模型 %q 返回了空响应。这通常意味着供应商配置有误，请检查：①Base URL 是否正确；②API Key 是否有效；③上游是否支持该模型名（注意模型映射）。",
		info.UpstreamModel,
	)
}

// statusHint 根据状态码返回中文可操作提示。
func statusHint(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "鉴权失败，请检查供应商 API Key"
	case status == http.StatusForbidden:
		return "无访问权限，请检查供应商账号权限或模型可用性"
	case status == http.StatusNotFound:
		return "上游资源不存在，请检查 Base URL 与模型名"
	case status == http.StatusTooManyRequests:
		return "触发上游限流（429），请稍后重试或降低请求频率"
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return "上游响应超时，请稍后重试"
	case status == http.StatusBadGateway:
		return "上游网关错误（502）"
	case status == http.StatusServiceUnavailable:
		return "上游服务不可用（503），请稍后重试"
	case status >= 500:
		return fmt.Sprintf("上游服务器错误（%d）", status)
	case status == http.StatusBadRequest:
		return "请求参数有误，请检查请求体与模型参数"
	default:
		return fmt.Sprintf("请求失败（HTTP %d）", status)
	}
}

// stripInternalPrefix 去除内部错误包装前缀（如 "stream: "、"upstream status 502: "、"do request: "），
// 避免把实现细节泄漏给客户端。
func stripInternalPrefix(s string) string {
	s = strings.TrimSpace(s)
	// 去除 "upstream status NNN: " 前缀
	if i := strings.Index(s, "upstream status "); i == 0 {
		if colon := strings.Index(s, ": "); colon != -1 {
			return strings.TrimSpace(s[colon+2:])
		}
	}
	// 去除常见内部包装前缀
	for _, p := range []string{"stream: ", "do request: ", "read upstream body: ", "convert response: "} {
		s = strings.TrimPrefix(s, p)
	}
	return strings.TrimSpace(s)
}

// truncateMessage 截断过长消息。
func truncateMessage(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
