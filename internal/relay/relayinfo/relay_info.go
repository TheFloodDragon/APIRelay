package relayinfo

import (
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/constant"
)

// RelayInfo 是单次中转尝试的上下文，供调度、转换、转发和日志共用。
type RelayInfo struct {
	RequestID      string
	StartTime      time.Time
	RelayMode      constant.RelayMode
	RelayFormat    constant.RelayFormat
	APIType        constant.APIType
	Channel        *model.Channel
	RequestedModel string
	ResolvedModel  string
	IsStream       bool
	ClientIP       string
}
