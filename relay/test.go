package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// ModelTestResult 模型连通性测试结果。
type ModelTestResult struct {
	Success  bool   `json:"success"`
	Model    string `json:"model"`     // 显示名
	Upstream string `json:"upstream"`  // 上游真实模型名
	Protocol string `json:"protocol"`  // 实际使用的上游协议名
	Latency  int64  `json:"latency_ms"`
	Reply    string `json:"reply"`     // 模型回复内容片段
	Usage    *dto.Usage `json:"usage,omitempty"`
	Error    string `json:"error,omitempty"`
}

// TestModel 对某渠道的指定显示模型发起一次最小化非流式对话，验证连通性。
//
// 复用统一转发链路：ResolveAPIType 选协议 -> Adaptor 转换/请求 -> ConvertResponse。
// 不计费、不写调用日志，仅用于管理后台的「测试」按钮。
func TestModel(ch *model.Channel, displayModel string) *ModelTestResult {
	res := &ModelTestResult{Model: displayModel}
	if ch == nil {
		res.Error = "供应商不存在"
		return res
	}

	apiType := ResolveAPIType(ch, displayModel)
	upstream := ch.MappedModel(displayModel)
	res.Upstream = upstream
	res.Protocol = constant.APITypeName(apiType)

	adp := GetAdaptor(apiType)
	if adp == nil {
		res.Error = "没有可用的适配器（协议：" + res.Protocol + "）"
		return res
	}

	info := &relaycommon.RelayInfo{
		EndpointType:  endpointForAPIType(apiType),
		ApiType:       apiType,
		Group:         ch.Group,
		Channel:       ch,
		OriginModel:   displayModel,
		UpstreamModel: upstream,
		IsStream:      false,
		StartAtMs:     time.Now().UnixMilli(),
	}
	adp.Init(info)

	// 构造最小测试请求
	maxTok := 16
	ir := &dto.UnifiedRequest{
		Model:     displayModel,
		MaxTokens: &maxTok,
		Messages: []dto.UnifiedMessage{
			{Role: dto.RoleUser, Content: "Say 'hi' in one word."},
		},
	}

	upstreamReq, err := adp.ConvertRequest(info, ir)
	if err != nil {
		res.Error = "构造请求失败：" + err.Error()
		return res
	}
	reqBody, err := json.Marshal(upstreamReq)
	if err != nil {
		res.Error = "序列化请求失败：" + err.Error()
		return res
	}

	start := time.Now()
	resp, err := adp.DoRequest(info, bytes.NewReader(reqBody))
	if err != nil {
		res.Error = "请求上游失败：" + err.Error()
		return res
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	res.Latency = time.Since(start).Milliseconds()

	if resp.StatusCode != 200 {
		res.Error = fmt.Sprintf("上游返回 %d：%s", resp.StatusCode, extractUpstreamErrorMessage(body))
		return res
	}

	uniResp, err := adp.ConvertResponse(info, body)
	if err != nil {
		res.Error = "解析响应失败：" + err.Error()
		return res
	}

	res.Success = true
	res.Reply = truncate(uniResp.Content, 200)
	if res.Reply == "" && len(uniResp.ToolCalls) > 0 {
		res.Reply = "(返回了工具调用)"
	}
	res.Usage = &uniResp.Usage
	return res
}

// endpointForAPIType 为测试构造一个与上游协议一致的对外端点类型，
// 使适配器走「同协议」路径，行为最接近真实调用。
func endpointForAPIType(t constant.APIType) constant.EndpointType {
	switch t {
	case constant.APITypeAnthropic:
		return constant.EndpointAnthropic
	case constant.APITypeResponses:
		return constant.EndpointResponses
	default:
		return constant.EndpointOpenAI
	}
}
