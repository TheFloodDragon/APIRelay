package openai

import (
	"bufio"
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

	// 检测是否可以零改写透传（对外也是 OpenAI 协议）
	canRaw := info.EndpointType == constant.EndpointOpenAI

	var usage *dto.Usage
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		
		// 空行：SSE 心跳
		if trimmed == "" {
			if canRaw {
				if err := onChunk(&dto.UnifiedStreamChunk{Raw: line}); err != nil {
					return usage, err
				}
			}
			continue
		}
		
		// 非 data: 行（event: 等）
		if !strings.HasPrefix(trimmed, "data:") {
			if canRaw {
				if err := onChunk(&dto.UnifiedStreamChunk{Raw: line}); err != nil {
					return usage, err
				}
			}
			continue
		}
		
		data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if data == "[DONE]" {
			break
		}
		
		// 解析以提取 usage
		chunk, err := apicompat.ParseOpenAIStreamChunk([]byte(data))
		if err != nil {
			// 解析失败：零改写透传（同协议）或跳过（跨协议）
			if canRaw {
				if err := onChunk(&dto.UnifiedStreamChunk{Raw: line}); err != nil {
					return usage, err
				}
			}
			continue
		}
		
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		
		// 同协议零改写，跨协议走 IR
		if canRaw {
			if err := onChunk(&dto.UnifiedStreamChunk{Raw: line}); err != nil {
				return usage, err
			}
		} else {
			if err := onChunk(chunk); err != nil {
				return usage, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return usage, err
	}
	return usage, nil
}
