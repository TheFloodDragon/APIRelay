package adaptor

import (
	"net/http"

	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
	"github.com/TheFloodDragon/APIRelay/internal/relay/protocol"
)

// Adaptor 负责在 APIRelay 入口格式与上游协议之间进行请求、响应和流式事件转换。
type Adaptor interface {
	APIType() constant.APIType
	GetRequestURL(baseURL string, mode constant.RelayMode) string
	SetupHeaders(headers http.Header, apiKey string, mode constant.RelayMode)
	ConvertRequest(req []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error)
	ConvertResponse(resp []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error)
	ConvertStreamChunk(chunk []byte, mode constant.RelayMode, format constant.RelayFormat) ([]byte, error)
	ErrorMessage(resp []byte) string
}

// RequestMetaAwareAdaptor 可在不改变基础接口的前提下接收请求元信息。
type RequestMetaAwareAdaptor interface {
	ConvertRequestWithMeta(req []byte, mode constant.RelayMode, format constant.RelayFormat, meta protocol.RequestMeta) ([]byte, error)
}

// ModelAwareURLAdaptor 是官方协议中 URL 依赖模型名的可选扩展。
// 它不改变 Adaptor 基础接口，当前主要用于 Gemini generateContent 路径。
type ModelAwareURLAdaptor interface {
	GetRequestURLWithModel(baseURL string, mode constant.RelayMode, model string, stream bool) string
}

// ConfigAwareHeaderAdaptor 可在不改变基础接口的前提下接收渠道配置来设置上游 Header。
type ConfigAwareHeaderAdaptor interface {
	SetupHeadersWithConfig(headers http.Header, apiKey string, mode constant.RelayMode, config map[string]interface{})
}
