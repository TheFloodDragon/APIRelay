package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/apicompat"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// defaultTestConcurrency 批量测试默认并发数。
const defaultTestConcurrency = 5

// defaultTestTimeout 单个模型测试的超时时间。
const defaultTestTimeout = 30 * time.Second

// ModelTestResult 模型连通性测试结果。
type ModelTestResult struct {
	Success  bool       `json:"success"`
	Model    string     `json:"model"`    // 显示名
	Upstream string     `json:"upstream"` // 上游真实模型名
	Protocol string     `json:"protocol"` // 实际使用的上游协议名
	Latency  int64      `json:"latency_ms"`
	Reply    string     `json:"reply"` // 模型回复内容片段
	Usage    *dto.Usage `json:"usage,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// TestModel 对某渠道的指定显示模型发起一次最小化非流式对话，验证连通性。
//
// 复用统一转发链路：ResolveAPIType 选协议 -> Adaptor 转换/请求 -> ConvertResponse。
// 不计费、不写调用日志，仅用于管理后台的「测试」按钮。
//
// 这是向后兼容的签名，等价于 TestModelWithContext(context.Background(), ...)。
func TestModel(ch *model.Channel, displayModel string) *ModelTestResult {
	return TestModelWithContext(context.Background(), ch, displayModel)
}

// TestModelWithContext 与 TestModel 相同，但使用调用方传入的 context，
// 以便批量测试为每个模型施加独立超时/取消。
func TestModelWithContext(ctx context.Context, ch *model.Channel, displayModel string) *ModelTestResult {
	res := &ModelTestResult{Model: displayModel}
	if ch == nil {
		res.Error = "供应商不存在"
		return res
	}
	if ctx == nil {
		ctx = context.Background()
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
		Context:       ctx,
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
	// Body 复写：与真实转发一致，在发往上游前深合并渠道配置的补丁。
	if patch := ch.SafeBodyOverride(); len(patch) > 0 {
		if merged, mErr := apicompat.ApplyBodyOverride(reqBody, patch); mErr == nil {
			reqBody = merged
		}
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

// TestModels 并发批量测试渠道下的多个显示模型。
//
// 使用带缓冲 channel 做并发限流（concurrency<=0 时用默认值），
// 每个模型在独立的 30s 超时 context 下执行。结果按 models 的输入顺序返回。
func TestModels(ctx context.Context, ch *model.Channel, models []string, concurrency int) []*ModelTestResult {
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]*ModelTestResult, len(models))
	if len(models) == 0 {
		return results
	}
	if concurrency <= 0 {
		concurrency = defaultTestConcurrency
	}
	if concurrency > len(models) {
		concurrency = len(models)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, m := range models {
		wg.Add(1)
		go func(idx int, displayModel string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mctx, cancel := context.WithTimeout(ctx, defaultTestTimeout)
			defer cancel()
			results[idx] = TestModelWithContext(mctx, ch, displayModel)
		}(i, m)
	}
	wg.Wait()
	return results
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
