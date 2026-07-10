package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/apirelay/apirelay/model"
)

// errEmptyUpstreamResponse 表示上游返回了空响应（常见于 base_url/模型/密钥配置错误）。
var errEmptyUpstreamResponse = errors.New("empty upstream response")

const statusClientClosedRequest = 499

type RelayErrorCategory string

const (
	ErrorCategoryClientCanceled  RelayErrorCategory = "client_canceled"
	ErrorCategoryRelayTimeout    RelayErrorCategory = "relay_timeout"
	ErrorCategoryUpstreamTimeout RelayErrorCategory = "upstream_timeout"
	// ErrorCategoryTimeout 保留为兼容旧调用方的泛化超时分类，新代码应优先使用更具体的分类。
	ErrorCategoryTimeout  RelayErrorCategory = "timeout"
	ErrorCategoryUpstream RelayErrorCategory = "upstream_error"
	ErrorCategoryInternal RelayErrorCategory = "internal_error"
)

// classifyRelayError 将客户端取消、请求总超时、上游超时和上游错误分开，避免客户端断开误伤渠道健康。
func classifyRelayError(ctx context.Context, err error) RelayErrorCategory {
	if ctx != nil {
		switch ctx.Err() {
		case context.Canceled:
			return ErrorCategoryClientCanceled
		case context.DeadlineExceeded:
			return ErrorCategoryRelayTimeout
		}
	}
	if err == nil {
		return ErrorCategoryUpstream
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCategoryClientCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorCategoryUpstreamTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrorCategoryUpstreamTimeout
	}

	msg := strings.ToLower(err.Error())
	// 仅将「客户端主动断开」归为客户端取消，避免误伤渠道健康。
	// 注意：connection reset by peer / broken pipe 属于上游/网络中断，
	// 必须归 upstream 以触发故障转移与熔断计数（见下方默认分支）。
	for _, part := range []string{
		"client disconnected",
		"client closed",
		"context canceled",
	} {
		if strings.Contains(msg, part) {
			return ErrorCategoryClientCanceled
		}
	}
	for _, part := range []string{
		"context deadline exceeded",
		"i/o timeout",
		"timeout awaiting response headers",
		"tls handshake timeout",
		"client.timeout exceeded",
	} {
		if strings.Contains(msg, part) {
			return ErrorCategoryUpstreamTimeout
		}
	}
	return ErrorCategoryUpstream
}

func isTimeoutCategory(category RelayErrorCategory) bool {
	switch category {
	case ErrorCategoryRelayTimeout, ErrorCategoryUpstreamTimeout, ErrorCategoryTimeout:
		return true
	default:
		return false
	}
}

func timeoutStatus(category RelayErrorCategory) int {
	if isTimeoutCategory(category) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

func timeoutLogMessage(category RelayErrorCategory) string {
	switch category {
	case ErrorCategoryRelayTimeout:
		return "relay request timeout"
	case ErrorCategoryUpstreamTimeout, ErrorCategoryTimeout:
		return "upstream timeout"
	default:
		return "upstream error"
	}
}

func noChannelError(group, modelName string) string {
	diag := model.DiagnoseModel(group, modelName)

	var b strings.Builder
	fmt.Fprintf(&b, "没有可用的渠道来服务模型 %q（分组 %q）。", modelName, group)

	switch {
	case len(diag.DisabledProviders) > 0:
		// 配置了但被禁用
		fmt.Fprintf(&b, "已配置该模型的供应商当前被禁用：%s。请在供应商管理中启用。",
			strings.Join(diag.DisabledProviders, "、"))
	case len(diag.OtherGroupProviders) > 0:
		// 分组不匹配
		fmt.Fprintf(&b, "该模型在其它分组下由供应商 %s 提供，但当前令牌分组为 %q，二者不匹配。请将供应商分组改为 %q，或改用对应分组的令牌。",
			strings.Join(diag.OtherGroupProviders, "、"), group, group)
	default:
		b.WriteString("请确认已配置启用该模型的供应商，且模型名拼写正确（区分大小写）。")
	}

	// 附：当前分组下可用的供应商与模型，便于排查
	if models, err := model.GetAvailableModels(group); err == nil && len(models) > 0 {
		shown := models
		const maxShow = 20
		suffix := ""
		if len(shown) > maxShow {
			shown = shown[:maxShow]
			suffix = fmt.Sprintf(" 等 %d 个", len(models))
		}
		fmt.Fprintf(&b, " 当前分组可用模型：%s%s。", strings.Join(shown, "、"), suffix)
	} else if diag.HasWildcard {
		b.WriteString(" 当前分组存在通配（*）供应商。")
	} else {
		fmt.Fprintf(&b, " 当前分组 %q 下暂无任何已启用的模型。", group)
	}

	return b.String()
}

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

// friendlyUpstreamError 为单渠道致命错误生成面向客户端的友好提示（含供应商名）。
func friendlyUpstreamError(info *RelayInfo, status int, err error) string {
	if errors.Is(err, errEmptyUpstreamResponse) {
		return providerPrefix(info) + emptyResponseHint(info)
	}
	detail := extractUpstreamErrorMessage([]byte(stripInternalPrefix(err.Error())))
	base := statusHint(status)
	if detail != "" {
		return fmt.Sprintf("%s%s（上游返回：%s）", providerPrefix(info), base, truncateMessage(detail, 300))
	}
	return providerPrefix(info) + base
}

// friendlyExhaustedError 为「所有渠道均失败」生成友好提示（含最后尝试的供应商名）。
func friendlyExhaustedError(info *RelayInfo, status int, lastErr string) string {
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	model := info.OriginModel
	if strings.Contains(lastErr, errEmptyUpstreamResponse.Error()) {
		return providerPrefix(info) + emptyResponseHint(info)
	}
	detail := extractUpstreamErrorMessage([]byte(stripInternalPrefix(lastErr)))
	base := fmt.Sprintf("模型 %q 的所有可用渠道均请求失败（%s）", model, statusHint(status))
	if detail != "" {
		return fmt.Sprintf("%s。最后一次错误来自供应商[%s]：%s", base, providerName(info), truncateMessage(detail, 300))
	}
	return base + "，请稍后重试或检查供应商配置。"
}

// providerName 返回当前（最后尝试）供应商名，未知时返回占位。
func providerName(info *RelayInfo) string {
	if info != nil && info.Channel != nil && info.Channel.Name != "" {
		return info.Channel.Name
	}
	return "未知"
}

// providerPrefix 返回 "供应商[名称] " 前缀（仅用于客户端响应）。
func providerPrefix(info *RelayInfo) string {
	if info != nil && info.Channel != nil && info.Channel.Name != "" {
		return "供应商[" + info.Channel.Name + "] "
	}
	return ""
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
	case status == statusClientClosedRequest:
		return "客户端已取消请求"
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

// cleanErrorMessage 生成用于【日志存储】的干净错误信息：
// 去除内部包装前缀，并尽量提取上游返回的可读消息（不附加供应商名，供应商在日志列体现）。
func cleanErrorMessage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, errEmptyUpstreamResponse.Error()) {
		return "上游返回空响应"
	}
	stripped := stripInternalPrefix(s)
	// 若剩余内容是 JSON 错误体，提取其中的 message
	if msg := extractUpstreamErrorMessage([]byte(stripped)); msg != "" {
		return truncateMessage(msg, 500)
	}
	return truncateMessage(stripped, 500)
}

// truncateMessage 截断过长消息。
func truncateMessage(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
