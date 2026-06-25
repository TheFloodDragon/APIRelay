package responses

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

// Adaptor 实现 OpenAI Responses 上游。
type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) ChannelTypeName() string { return "OpenAI-Responses" }

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimRight(info.Channel.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com"
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/responses", nil
	}
	return base + "/v1/responses", nil
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
	return apicompat.BuildResponsesRequest(ir, info.UpstreamModel), nil
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
	var resp dto.ResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse responses response: %w", err)
	}
	return apicompat.ResponsesResponseToIR(&resp), nil
}

func (a *Adaptor) StreamHandler(info *relaycommon.RelayInfo, resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error) {
	scanner := bufio.NewScanner(resp.Body)
	var usage *dto.Usage

	err := adaptor.ScanSSE(scanner, func(data string) error {
		chunk, perr := apicompat.ParseResponsesStreamEvent([]byte(data))
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
