package adaptor

import (
	"fmt"
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

// ProviderAdapter 是透传优先转发路径使用的新适配器接口。
// 它在保留旧 Adaptor 能力的基础上，显式暴露协议是否需要转换的判断，
// 让控制器可以在入口格式与上游格式一致时跳过序列化/反序列化转换。
//
// 注意：鉴权方法使用基础类型而不是 AuthInfo 结构体，是为了避免父级 adaptor 包
// 与 openai/anthropic/gemini 子包之间形成 import cycle。
type ProviderAdapter interface {
	Adaptor
	Name() string
	ExtractBaseURL(channel *model.Channel) (string, error)
	ExtractAuth(channel *model.Channel) (apiKey string, config map[string]interface{})
	BuildURL(baseURL string, mode constant.RelayMode, resolvedModel string, stream bool) string
	GetAuthHeaders(apiKey string, config map[string]interface{}, mode constant.RelayMode, stream bool) (http.Header, error)
	NeedsTransform(channel *model.Channel, callerFormat constant.RelayFormat) bool
}

// AsProviderAdapter 返回实现新接口的适配器。
// 如果某个旧适配器尚未迁移，则使用兼容包装器提供基础能力。
func AsProviderAdapter(protocolAdaptor Adaptor) ProviderAdapter {
	if providerAdaptor, ok := protocolAdaptor.(ProviderAdapter); ok {
		return providerAdaptor
	}
	return &legacyProviderAdapter{Adaptor: protocolAdaptor}
}

type legacyProviderAdapter struct {
	Adaptor
}

func (a *legacyProviderAdapter) Name() string {
	if a == nil || a.Adaptor == nil {
		return "unknown"
	}
	return a.APIType().String()
}

func (a *legacyProviderAdapter) ExtractBaseURL(channel *model.Channel) (string, error) {
	if channel == nil {
		return "", fmt.Errorf("channel is nil")
	}
	return channel.BaseURL, nil
}

func (a *legacyProviderAdapter) ExtractAuth(channel *model.Channel) (string, map[string]interface{}) {
	if channel == nil {
		return "", nil
	}
	return channel.APIKey, channel.Config
}

func (a *legacyProviderAdapter) BuildURL(baseURL string, mode constant.RelayMode, resolvedModel string, stream bool) string {
	if urlAdaptor, ok := a.Adaptor.(ModelAwareURLAdaptor); ok {
		return urlAdaptor.GetRequestURLWithModel(baseURL, mode, resolvedModel, stream)
	}
	return a.GetRequestURL(baseURL, mode)
}

func (a *legacyProviderAdapter) GetAuthHeaders(apiKey string, config map[string]interface{}, mode constant.RelayMode, stream bool) (http.Header, error) {
	headers := http.Header{}
	if configAware, ok := a.Adaptor.(ConfigAwareHeaderAdaptor); ok {
		configAware.SetupHeadersWithConfig(headers, apiKey, mode, config)
	} else {
		a.SetupHeaders(headers, apiKey, mode)
	}
	if stream {
		headers.Set("Accept", "text/event-stream")
	}
	return headers, nil
}

func (a *legacyProviderAdapter) NeedsTransform(channel *model.Channel, callerFormat constant.RelayFormat) bool {
	if a == nil || a.Adaptor == nil {
		return true
	}
	switch a.APIType() {
	case constant.APITypeAnthropic:
		return callerFormat != constant.RelayFormatAnthropic
	case constant.APITypeGemini:
		return callerFormat != constant.RelayFormatGemini
	case constant.APITypeOpenAI:
		return callerFormat != constant.RelayFormatOpenAI && callerFormat != constant.RelayFormatOpenAIResponses
	default:
		return true
	}
}
