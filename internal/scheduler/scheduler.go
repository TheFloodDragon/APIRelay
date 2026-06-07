package scheduler

import (
	"fmt"

	"github.com/yourusername/apirelay/internal/model"
	"github.com/yourusername/apirelay/internal/repository"
)

// Scheduler 调度器
type Scheduler struct {
	channelRepo *repository.ChannelRepository
	strategy    string
}

func NewScheduler(channelRepo *repository.ChannelRepository, strategy string) *Scheduler {
	if strategy == "" {
		strategy = "priority"
	}
	return &Scheduler{
		channelRepo: channelRepo,
		strategy:    strategy,
	}
}

// SelectChannel 选择可用渠道
func (s *Scheduler) SelectChannel(modelName string) (*model.Channel, error) {
	channels, err := s.channelRepo.GetByModel(modelName)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, fmt.Errorf("没有找到支持模型 %s 的渠道", modelName)
	}

	// 过滤健康的渠道
	healthyChannels := make([]model.Channel, 0)
	for _, ch := range channels {
		if ch.HealthStatus != "unhealthy" {
			healthyChannels = append(healthyChannels, ch)
		}
	}

	if len(healthyChannels) == 0 {
		return nil, fmt.Errorf("所有支持模型 %s 的渠道都不可用", modelName)
	}

	// 按优先级策略选择
	switch s.strategy {
	case "priority":
		return &healthyChannels[0], nil
	case "weighted":
		// TODO: 实现加权随机选择
		return &healthyChannels[0], nil
	case "round_robin":
		// TODO: 实现轮询选择
		return &healthyChannels[0], nil
	default:
		return &healthyChannels[0], nil
	}
}

// GetAllChannelsForModel 获取支持某模型的所有渠道（用于失败重试）
func (s *Scheduler) GetAllChannelsForModel(modelName string) ([]model.Channel, error) {
	channels, err := s.channelRepo.GetByModel(modelName)
	if err != nil {
		return nil, err
	}

	// 过滤健康的渠道
	healthyChannels := make([]model.Channel, 0)
	for _, ch := range channels {
		if ch.HealthStatus != "unhealthy" {
			healthyChannels = append(healthyChannels, ch)
		}
	}

	return healthyChannels, nil
}
