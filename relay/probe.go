package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	url := modelsURL(ch)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setProbeAuth(ch, req.Header)
	for k, v := range ch.SafeHeaderOverrideMap() {
		req.Header.Set(k, v)
	}

	client := adaptor.HTTPClient()
	ctxClient := *client
	ctxClient.Timeout = 20 * time.Second
	resp, err := ctxClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request upstream models: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	return parseModelsBody(body)
}

func modelsURL(ch *model.Channel) string {
	base := strings.TrimRight(ch.BaseURL, "/")
	if base == "" {
		switch ch.APIType() {
		case constant.APITypeAnthropic:
			base = "https://api.anthropic.com"
		default:
			base = "https://api.openai.com"
		}
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/models"
	}
	return base + "/v1/models"
}

func setProbeAuth(ch *model.Channel, h http.Header) {
	if ch.Key == "" {
		return
	}
	switch ch.APIType() {
	case constant.APITypeAnthropic:
		h.Set("x-api-key", ch.Key)
		h.Set("anthropic-version", "2023-06-01")
	default:
		h.Set("Authorization", "Bearer "+ch.Key)
	}
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
