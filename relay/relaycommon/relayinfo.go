package relaycommon

import (
	"context"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
)

// RelayInfo 贯穿一次转发请求的上下文信息。
// 放在独立子包，供 relay 主包与各 adaptor 子包共享，避免循环依赖。
type RelayInfo struct {
	// Context 绑定客户端取消与 relay.request_timeout，用于中止上游请求和重试等待。
	Context context.Context

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

	// 计费：预扣的额度（微美元），用于结算时调整差额
	ReservedQuota int64
	// Settled 标记本次请求是否已完成额度结算（成功路径置 true）
	Settled bool

	// 计时
	StartAtMs   int64
	FirstByteMs int
}
