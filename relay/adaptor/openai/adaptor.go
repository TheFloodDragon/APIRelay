package openai

import (
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
	for k, v := range info.Channel.SafeHeaderOverrideMap() {
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
	// 解压上游响应（gzip/deflate）
	adaptor.DecompressResponse(resp)
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
	// 同协议（对外也是 OpenAI Chat）：逐行原样透传，保留 data:/空行/[DONE]。
	canRaw := info.EndpointType == constant.EndpointOpenAI

	var usage *dto.Usage
	lineCount := 0
	err := adaptor.StreamRawLines(resp.Body, func(line string) error {
		lineCount++
		data, isData := adaptor.ParseSSEData(line)

		// 旁路解析 data 行提取 usage（[DONE] 与空 data 跳过）
		if isData && data != "" && data != "[DONE]" {
			if chunk, perr := apicompat.ParseOpenAIStreamChunk([]byte(data)); perr == nil && chunk != nil {
				if chunk.Usage != nil {
					usage = chunk.Usage
				}
				// 跨协议：转换为 IR 下发
				if !canRaw {
					return onChunk(chunk)
				}
			} else if !canRaw {
				// 跨协议解析失败：跳过该行（可能是无法映射的特殊字段）
				return nil
			}
		} else if !canRaw {
			// 跨协议时，非 data 行（event:/空行/[DONE]）无需下发
			return nil
		}

		// 同协议：原样转发每一行
		return onChunk(&dto.UnifiedStreamChunk{Raw: line, IsRaw: true})
	})
	if err != nil {
		return usage, fmt.Errorf("stream after %d lines: %w", lineCount, err)
	}
	return usage, nil
}
