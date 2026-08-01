package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/adaptor"
)

// ProbeModels 按渠道协议调用上游标准模型列表接口，返回模型 ID 列表。
//
// 各协议的模型列表端点：
//   - OpenAI / Responses: GET {base}/v1/models  -> {"data":[{"id":...}]}
//   - Anthropic:          GET {base}/v1/models  -> {"data":[{"id":...}]}
func ProbeModels(ch *model.Channel) ([]string, error) {
	return ProbeModelsContext(context.Background(), ch)
}

// ProbeModelsContext 带 context 的模型探测，支持调用方取消。
func ProbeModelsContext(ctx context.Context, ch *model.Channel) ([]string, error) {
	if ch == nil {
		return nil, fmt.Errorf("channel is nil")
	}

	// 中转站的 base_url 常带 /anthropic、/claudecode 这类兼容后缀，或使用
	// /paas/v4 这种非 /v1 的版本段。单一拼接规则在这些情况下必然 404，
	// 因此按候选列表依次尝试，返回首个成功结果。
	candidates := modelsURLCandidates(ch)
	var lastErr error
	var lastStatus int
	var lastBody []byte

	for _, candidate := range candidates {
		status, body, err := probeModelsOnce(ctx, ch, candidate, false)
		if err != nil {
			// 目标被 SSRF 守卫拒绝时，换 URL 也不会有帮助，直接返回。
			if isBlockedProbeErr(err) {
				return nil, err
			}
			lastErr = err
			continue
		}

		// 部分 OpenAI 兼容聚合服务的对话端点支持 Anthropic x-api-key，
		// 但模型列表端点仍只接受 Authorization: Bearer。鉴权失败时换一种
		// 标准鉴权方式重试一次，避免要求用户为了模型探测修改渠道协议。
		if (status == http.StatusUnauthorized || status == http.StatusForbidden) && strings.TrimSpace(ch.Key) != "" {
			altStatus, altBody, altErr := probeModelsOnce(ctx, ch, candidate, true)
			if altErr == nil {
				status, body = altStatus, altBody
			}
		}

		if status == http.StatusOK {
			models, parseErr := parseModelsBody(body)
			if parseErr == nil {
				return models, nil
			}
			// 状态 200 但解析不出模型：可能命中了同名的其它端点，继续试下一个候选。
			lastErr = parseErr
			lastStatus, lastBody = status, body
			continue
		}
		lastStatus, lastBody = status, body
		lastErr = nil
	}

	if lastStatus != 0 {
		return nil, fmt.Errorf("upstream status %d: %s", lastStatus, truncate(string(lastBody), 300))
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("cannot parse models from upstream response")
}

func isBlockedProbeErr(err error) bool {
	var blocked *adaptor.ErrBlockedProbeTarget
	return errors.As(err, &blocked)
}

func probeModelsOnce(ctx context.Context, ch *model.Channel, rawURL string, alternateAuth bool) (int, []byte, error) {
	// SSRF 防护：base_url 完全由管理端输入，若不校验，这个接口就成了内网探测代理
	// （既能扫内网端口，也能读 169.254.169.254 上的云实例元数据）。
	if err := validateProbeTarget(ctx, rawURL); err != nil {
		return 0, nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	setProbeAuth(ch, req.Header, alternateAuth)
	for k, v := range ch.SafeHeaderOverrideMap() {
		req.Header.Set(k, v)
	}

	// 用带 SSRF 守卫的专用客户端，建连层再校验一次实际目标 IP，
	// 消除「校验时返回公网、连接时返回内网」的 DNS rebinding 窗口。
	client, closeIdle, err := adaptor.ProbeHTTPClient(probeTimeout)
	if err != nil {
		return 0, nil, err
	}
	defer closeIdle()

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request upstream models: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, body, nil
}

// probeTimeout 探测请求的整体超时。
const probeTimeout = 20 * time.Second

// ValidateChannelProbeTarget 校验渠道的 base_url 是否允许作为探测/测试目标。
// 供 controller 在处理未保存的临时配置时前置调用。
func ValidateChannelProbeTarget(ctx context.Context, ch *model.Channel) error {
	if ch == nil {
		return fmt.Errorf("channel is nil")
	}
	base := strings.TrimSpace(ch.BaseURL)
	if base == "" {
		return nil // 留空时使用官方默认地址，无需校验
	}
	return validateProbeTarget(ctx, base)
}

// validateProbeTarget 校验 URL 的 scheme 与主机是否允许作为探测目标。
func validateProbeTarget(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("目标地址无效: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		// 阻止 file://、gopher:// 等可用于读本地文件或打内网协议的 scheme。
		return fmt.Errorf("目标地址协议 %q 不受支持，只允许 http/https", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("目标地址缺少主机名")
	}
	return adaptor.ValidateProbeHost(ctx, parsed.Hostname())
}

// knownCompatSuffixes 是中转站常用的协议兼容路径后缀。
// base_url 以这些后缀结尾时，模型列表端点通常挂在剥掉后缀的根上，
// 例如 https://x.com/anthropic -> https://x.com/v1/models。
//
// 按长度降序排列：/api/claudecode 必须先于 /claudecode 匹配，
// 否则只会剥掉后半段，留下无意义的 /api 前缀。
var knownCompatSuffixes = []string{
	"/api/claudecode",
	"/compatible-mode",
	"/claudecode",
	"/anthropic",
	"/openai",
	"/coding",
	"/claude",
	"/codex",
}

// versionSegmentPrefixes 用于识别 base_url 末段是否已是 API 版本段。
// 命中时直接在其后拼 /models，不再插入 /v1（如智谱 /paas/v4 -> /paas/v4/models）。
var versionSegmentPrefixes = []string{"v1", "v2", "v3", "v4", "v1beta", "v1alpha"}

func defaultProbeBase(ch *model.Channel) string {
	switch ch.APIType() {
	case constant.APITypeAnthropic:
		return "https://api.anthropic.com"
	default:
		return "https://api.openai.com"
	}
}

// modelsURL 返回首选的模型列表地址（保留供单一地址场景使用）。
func modelsURL(ch *model.Channel) string {
	return modelsURLCandidates(ch)[0]
}

// modelsURLCandidates 生成有序去重的候选模型列表地址。
//
// 顺序即优先级：越贴合 base_url 原始形态的排在前面，逐步放宽到剥离兼容后缀。
func modelsURLCandidates(ch *model.Channel) []string {
	base := strings.TrimRight(strings.TrimSpace(ch.BaseURL), "/")
	if base == "" {
		base = defaultProbeBase(ch)
	}

	var candidates []string
	add := func(u string) {
		if u == "" {
			return
		}
		for _, existing := range candidates {
			if existing == u {
				return
			}
		}
		candidates = append(candidates, u)
	}

	// 末段已是版本段（/v1、/paas/v4、/v1beta 等）：直接拼 /models。
	if isVersionSegment(lastPathSegment(base)) {
		add(base + "/models")
	} else {
		add(base + "/v1/models")
		// 少数上游把模型列表挂在根下且不带版本段。
		add(base + "/models")
	}

	// base_url 带兼容后缀时，剥掉后缀再试一轮。
	// 例如 https://x.com/anthropic -> https://x.com/v1/models
	// 只取最长匹配的一个后缀：/api/claudecode 若同时按 /claudecode 剥离，
	// 会留下无意义的 /api 前缀并产生错误候选。
	lower := strings.ToLower(base)
	for _, suffix := range knownCompatSuffixes {
		if !strings.HasSuffix(lower, suffix) {
			continue
		}
		trimmed := strings.TrimRight(base[:len(base)-len(suffix)], "/")
		if trimmed != "" {
			if isVersionSegment(lastPathSegment(trimmed)) {
				add(trimmed + "/models")
			} else {
				add(trimmed + "/v1/models")
			}
		}
		break
	}

	return candidates
}

func lastPathSegment(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func isVersionSegment(segment string) bool {
	segment = strings.ToLower(segment)
	for _, prefix := range versionSegmentPrefixes {
		if segment == prefix {
			return true
		}
	}
	return false
}

func setProbeAuth(ch *model.Channel, h http.Header, alternate bool) {
	if ch.Key == "" {
		return
	}
	useAPIKey := ch.APIType() == constant.APITypeAnthropic
	if alternate {
		useAPIKey = !useAPIKey
	}
	if useAPIKey {
		h.Set("x-api-key", ch.Key)
		h.Set("anthropic-version", "2023-06-01")
		return
	}
	h.Set("Authorization", "Bearer "+ch.Key)
}

// parseModelsBody 兼容多种返回结构：
//
//	{"data":[{"id":"..."}]}（OpenAI/Anthropic 标准）
//	{"models":[{"id":"..."}]} 或 ["id1","id2"]（兜底）
func parseModelsBody(body []byte) ([]string, error) {
	type modelItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var std struct {
		Data   []modelItem `json:"data"`
		Models []modelItem `json:"models"`
	}
	if err := json.Unmarshal(body, &std); err == nil {
		items := std.Data
		if len(items) == 0 {
			items = std.Models
		}
		var out []string
		for _, it := range items {
			id := it.ID
			if id == "" {
				id = it.Name
			}
			if id != "" {
				out = append(out, id)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	// 纯字符串数组
	var arr []string
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		return arr, nil
	}

	return nil, fmt.Errorf("cannot parse models from upstream response")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
