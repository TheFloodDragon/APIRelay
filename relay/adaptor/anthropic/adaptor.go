package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/relay/adaptor"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// Adaptor 实现 Anthropic Messages 上游。
type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ChannelTypeName() string { return "Anthropic" }

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimRight(info.Channel.BaseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages", nil
	}
	return base + "/v1/messages", nil
}

func (a *Adaptor) SetupRequestHeader(info *relaycommon.RelayInfo, h http.Header) error {
	h.Set("Content-Type", "application/json")
	h.Set("anthropic-version", "2023-06-01")
	if info.Channel.Key != "" {
		h.Set("x-api-key", info.Channel.Key)
	}
	if info.IsStream {
		h.Set("Accept", "text/event-stream")
	}
	// 渠道自定义请求头（可注入 Claude Code wire image 等）
	for k, v := range info.Channel.SafeHeaderOverrideMap() {
		h.Set(k, v)
	}
	return nil
}

func (a *Adaptor) ConvertRequest(info *relaycommon.RelayInfo, ir *dto.UnifiedRequest) (any, error) {
	return apicompat.BuildAnthropicRequest(ir, info.UpstreamModel), nil
}

func (a *Adaptor) DoRequest(info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	url, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}
	ctx := info.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
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
	// 解压上游响应（gzip/deflate），避免压缩流导致解析与透传乱码
	adaptor.DecompressResponse(resp)
	if id := resp.Header.Get("request-id"); id != "" {
		info.UpstreamRequestId = id
	}
	return resp, nil
}

func (a *Adaptor) ConvertResponse(info *relaycommon.RelayInfo, body []byte) (*dto.UnifiedResponse, error) {
	var resp dto.AnthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse anthropic response: %w", err)
	}
	return apicompat.AnthropicResponseToIR(&resp), nil
}

func (a *Adaptor) StreamHandler(info *relaycommon.RelayInfo, resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error) {
	// 同协议（对外也是 Anthropic）：逐行原样透传，完整保留 event: 行、空行、
	// data: 行等 SSE 结构，避免客户端（如 Claude Code）无法解析事件类型。
	if info.EndpointType == constant.EndpointAnthropic {
		return a.streamRaw(resp, onChunk)
	}
	// 跨协议：解析为 IR，由出站侧重新序列化。
	return a.streamIR(resp, onChunk)
}

// streamRaw 逐行原样转发，同时旁路解析 data 行以提取 usage。
func (a *Adaptor) streamRaw(resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error) {
	parser := apicompat.NewAnthropicStreamParser()
	var usage *dto.Usage

	err := adaptor.StreamRawLines(resp.Body, func(line string) error {
		// 旁路解析：仅读取 data 行提取 usage，不影响转发
		if data, ok := adaptor.ParseSSEData(line); ok && data != "" && data != "[DONE]" {
			if chunk, perr := parser.Parse([]byte(data)); perr == nil && chunk != nil && chunk.Usage != nil {
				usage = chunk.Usage
			}
		}
		// 原样转发每一行（含 event:/空行/data:/[DONE]）
		return onChunk(&dto.UnifiedStreamChunk{Raw: line, IsRaw: true})
	})
	return usage, err
}

// streamIR 跨协议解析路径：只读 data 行，转换为统一增量。
func (a *Adaptor) streamIR(resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error) {
	parser := apicompat.NewAnthropicStreamParser()
	scanner := bufio.NewScanner(resp.Body)
	var usage *dto.Usage

	err := adaptor.ScanSSE(scanner, func(data string) error {
		chunk, perr := parser.Parse([]byte(data))
		if perr != nil || chunk == nil {
			return nil
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		return onChunk(chunk)
	})
	return usage, err
}
