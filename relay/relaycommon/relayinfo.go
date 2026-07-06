package relaycommon

import (
	"context"
	"sync/atomic"

	"github.com/apirelay/apirelay/constant"
	"github.com/apirelay/apirelay/model"
)

const unsetChannelMaxRetries int64 = -1

var runtimeChannelMaxRetries atomic.Int64

func init() {
	runtimeChannelMaxRetries.Store(unsetChannelMaxRetries)
}

// SetRuntimeChannelMaxRetries 更新运行时单渠道重试次数。传入负数会恢复为使用启动配置。
func SetRuntimeChannelMaxRetries(retries int) {
	if retries < 0 {
		runtimeChannelMaxRetries.Store(unsetChannelMaxRetries)
		return
	}
	runtimeChannelMaxRetries.Store(int64(retries))
}

// RuntimeChannelMaxRetries 返回当前运行时单渠道重试次数；未在线覆盖时返回 fallback。
func RuntimeChannelMaxRetries(fallback int) int {
	if retries := runtimeChannelMaxRetries.Load(); retries >= 0 {
		return int(retries)
	}
	return fallback
}

// RelayInfo 贯穿一次转发请求的上下文信息。
// 放在独立子包，供 relay 主包与各 adaptor 子包共享，避免循环依赖。
type RelayInfo struct {
	// Context 绑定客户端取消与 relay.request_timeout，用于中止上游请求和重试等待。
	Context context.Context

	RequestID string
	ClientIP  string

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

	// FailoverChain JSON：记录本次请求尝试过的渠道、错误与决策链，写入日志 Content 便于诊断。
	FailoverChain string

	// 计时
	StartAtMs   int64
	FirstByteMs int
}
