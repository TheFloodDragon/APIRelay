package adaptor

import (
	"io"
	"net/http"

	"github.com/apirelay/apirelay/dto"
	"github.com/apirelay/apirelay/relay/relaycommon"
)

// Adaptor 是所有上游协议适配器的统一接口。
// 它接收内部 IR (UnifiedRequest)，转换为上游协议并完成请求/响应处理。
type Adaptor interface {
	// Init 在选定渠道后初始化适配器状态。
	Init(info *relaycommon.RelayInfo)
	// GetRequestURL 返回上游完整请求地址。
	GetRequestURL(info *relaycommon.RelayInfo) (string, error)
	// SetupRequestHeader 设置上游请求头（含鉴权）。
	SetupRequestHeader(info *relaycommon.RelayInfo, h http.Header) error
	// ConvertRequest 将 IR 转换为上游协议请求体（返回可被 json.Marshal 的结构）。
	ConvertRequest(info *relaycommon.RelayInfo, ir *dto.UnifiedRequest) (any, error)
	// DoRequest 发送上游请求。
	DoRequest(info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error)
	// ConvertResponse 将上游非流式响应体转换为统一响应。
	ConvertResponse(info *relaycommon.RelayInfo, body []byte) (*dto.UnifiedResponse, error)
	// StreamHandler 处理上游流式响应，逐事件回调统一增量。
	// onChunk 返回 error 时应中止读取。
	StreamHandler(info *relaycommon.RelayInfo, resp *http.Response, onChunk func(*dto.UnifiedStreamChunk) error) (*dto.Usage, error)
	// ChannelTypeName 适配器名称（用于日志）。
	ChannelTypeName() string
}
