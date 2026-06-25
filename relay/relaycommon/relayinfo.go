package relaycommon

import (
	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
)

// RelayInfo 贯穿一次转发请求的上下文信息。
// 放在独立子包，供 relay 主包与各 adaptor 子包共享，避免循环依赖。
type RelayInfo struct {
	RequestID    string
	EndpointType constant.EndpointType // 对外协议
	ApiType      constant.APIType      // 上游协议
	Group        string

	// 鉴权得到的令牌信息
	TokenId   int
	TokenName string
	UserId    int

	// 模型
	OriginModel   string // 客户端请求的模型
	UpstreamModel string // 经 model_mapping 映射后的真实模型

	// 选中的渠道
	Channel *model.Channel

	IsStream bool

	// 上游请求相关
	UpstreamRequestId string

	// 计时
	StartAtMs   int64
	FirstByteMs int
}
