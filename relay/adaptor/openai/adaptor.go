package openai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// Adaptor 实现 OpenAI Chat Completions 兼容上游。
type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ChannelTypeName() string { return "OpenAI" }

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimRight(info.Channel.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	// 若用户已填到 /v1 则不重复追加
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions", nil
	}
	return base + "/v1/chat/completions", nil
}

func (a *Adaptor) SetupRequestHeader(info *relaycommon.RelayInfo, h http.Header) error {
	h.Set("Content-Type", "application/json")
	if info.Channel.Key != "" {
		h.Set("Authorization", "Bearer "+info.Channel.Key)
	}
	if info.IsStream {
		h.Set("Accept", "text/event-stream")
	}
	for k, v := range info.Channel.HeaderOverrideMap() {
		h.Set(k, v)
	}
	return nil
}

func (a *Adaptor) ConvertRequest(info *relaycommon.RelayInfo, ir *dto.UnifiedRequest) (any, error) {
	return apicompat.BuildOpenAIRequest(ir, info.UpstreamModel), nil
}

func (a *Adaptor) DoRequest(info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if err := a.SetupRequestHeader(info, req.Header); err != nil {
		return nil, err
	}
	resp, err := adaptor.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	if id := resp.Header.Get("X-Request-Id"); id != "" {
		info.UpstreamRequestId = id
	}
	return resp, nil
}

func (a *Adaptor) ConvertResponse(info *relaycommon.RelayInfo, body []byte) (*dto.UnifiedResponse, error) {
	var resp dto.OpenAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}
	return apicompat.OpenAIResponseToIR(&resp), nil
}

func (a *Adaptor) StreamHandler(info *relaycommon.RelayInfo, resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var usage *dto.Usage
	for scanner.Scan() {
		line := scanner.Text() // 保留原始行（不 TrimSpace），包括可能的心跳空行
		
		// 空行：SSE 心跳，透传
		if strings.TrimSpace(line) == "" {
			// 空行对维持长连接很重要，但无需处理，继续扫描
			continue
		}
		
		// 非 data: 行（如注释、event: 等）：透传原样
		if !strings.HasPrefix(strings.TrimSpace(line), "data:") {
			// 可能是 event: xxx 或其他 SSE 控制行，目前跳过
			// 如果需要严格透传，后续可改为写回客户端
			continue
		}
		
		data := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
		if data == "[DONE]" {
			// [DONE] 标记由 Finish() 统一发送，这里直接跳出
			break
		}
		
		chunk, err := apicompat.ParseOpenAIStreamChunk([]byte(data))
		if err != nil {
			// 无法解析的 chunk：可能是特殊事件（工具调用等）
			// 创建一个空 chunk 让下游能收到信号，避免完全丢弃
			// TODO: 引入 RawChunk 模式后，这里应透传原始 line
			chunk = &dto.UnifiedStreamChunk{} // 空 chunk，避免 nil
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if err := onChunk(chunk); err != nil {
			return usage, err
		}
	}
	if err := scanner.Err(); err != nil {
		return usage, err
	}
	return usage, nil
}
