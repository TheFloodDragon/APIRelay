package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/apirelay/apirelay/model"
)

// FailoverDecision 表示一次失败后的处置决策。
type FailoverDecision int

const (
	// DecisionRetrySameChannel 在同一渠道上重试（临时性错误，如 429/503）。
	DecisionRetrySameChannel FailoverDecision = iota
	// DecisionSwitchChannel 冷却当前渠道并切换到其它渠道。
	DecisionSwitchChannel
	// DecisionFatal 不可重试，直接返回错误。
	DecisionFatal
)

// FailoverAttempt 记录一次渠道尝试及其调度决策，用于最终日志诊断。
type FailoverAttempt struct {
	Iter          int    `json:"iter"`
	Switches      int    `json:"switches"`
	ChannelId     int    `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ApiType       string `json:"api_type"`
	OriginModel   string `json:"origin_model"`
	UpstreamModel string `json:"upstream_model"`
	Status        int    `json:"status"`
	Retryable     bool   `json:"retryable"`
	Decision      string `json:"decision"`
	ErrorCategory string `json:"error_category,omitempty"`
	Error         string `json:"error,omitempty"`
	AtMs          int64  `json:"at_ms"`
}

// FailoverState 跨重试迭代共享的故障转移状态（借鉴 sub2api FailoverState）。
type FailoverState struct {
	// FailedChannels 已被切换排除的渠道集合。
	FailedChannels map[int]struct{}
	// SameChannelRetries 每个渠道的同渠道重试次数。
	SameChannelRetries map[int]int

	maxSameChannelRetries int
	cooldownSeconds       int

	LastStatus int
	LastErr    string
	Attempts   []FailoverAttempt
}

const (
	defaultMaxSameChannelRetries = 2
	sameChannelRetryDelay        = 400 * time.Millisecond
)

// NewFailoverState 创建故障转移状态。
func NewFailoverState(cooldownSeconds, channelMaxRetries int) *FailoverState {
	if channelMaxRetries < 0 {
		channelMaxRetries = defaultMaxSameChannelRetries
	}
	return &FailoverState{
		FailedChannels:        make(map[int]struct{}),
		SameChannelRetries:    make(map[int]int),
		maxSameChannelRetries: channelMaxRetries,
		cooldownSeconds:       cooldownSeconds,
	}
}

// Decide 根据上游状态码与是否可重试，决定下一步处置。
// 已废弃：请使用 OnFailure。保留以兼容潜在调用方。

// OnFailure 记录一次失败并返回处置决策。
//   - retryable=false：致命错误，直接返回。
//   - 临时性错误（429/503/网络）且同渠道重试次数未耗尽：同渠道重试。
//   - 否则：冷却并切换渠道。
func (s *FailoverState) OnFailure(channelID, status int, retryable bool, errMsg string) FailoverDecision {
	s.LastStatus, s.LastErr = status, errMsg
	if !retryable {
		return DecisionFatal
	}

	if isTransientStatus(status) && s.SameChannelRetries[channelID] < s.maxSameChannelRetries {
		s.SameChannelRetries[channelID]++
		return DecisionRetrySameChannel
	}

	// 切换渠道：冷却并排除
	s.FailedChannels[channelID] = struct{}{}
	model.SetChannelCooldown(channelID, time.Now().Add(time.Duration(s.cooldownSeconds)*time.Second).UnixMilli())
	return DecisionSwitchChannel
}

func (s *FailoverState) RecordAttempt(a FailoverAttempt) {
	if a.AtMs == 0 {
		a.AtMs = time.Now().UnixMilli()
	}
	if a.Error != "" {
		a.Error = truncateMessage(cleanErrorMessage(a.Error), 500)
	}
	s.Attempts = append(s.Attempts, a)
}

func (s *FailoverState) ChainJSON() string {
	if s == nil || len(s.Attempts) == 0 {
		return ""
	}
	b, err := json.Marshal(s.Attempts)
	if err != nil {
		return ""
	}
	return string(b)
}

func failoverDecisionLabel(d FailoverDecision) string {
	switch d {
	case DecisionRetrySameChannel:
		return "retry_same_channel"
	case DecisionSwitchChannel:
		return "switch_channel"
	case DecisionFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// Excluded 返回当前应排除的渠道集合（用于下一次选渠道）。
func (s *FailoverState) Excluded() map[int]struct{} {
	return s.FailedChannels
}

// SameChannelDelay 在同渠道重试前等待（带 context 取消）。返回 false 表示被取消。
func (s *FailoverState) SameChannelDelay(ctx context.Context) bool {
	t := time.NewTimer(sameChannelRetryDelay)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// isTransientStatus 判断是否为适合"同渠道重试"的瞬时错误。
func isTransientStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429 限流
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	}
	return false
}
