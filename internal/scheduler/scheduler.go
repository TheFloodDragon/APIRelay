package scheduler

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

// Scheduler 调度器
type Scheduler struct {
	channelRepo *repository.ChannelRepository
	strategy    string
	mu          sync.Mutex
	rrCounters  map[string]int
}

func NewScheduler(channelRepo *repository.ChannelRepository, strategy string) *Scheduler {
	if strategy == "" {
		strategy = "priority"
	}
	return &Scheduler{
		channelRepo: channelRepo,
		strategy:    strategy,
		rrCounters:  make(map[string]int),
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

	// 按策略选择
	switch s.strategy {
	case "weighted":
		return selectWeighted(healthyChannels), nil
	case "round_robin":
		return s.selectRoundRobin(modelName, healthyChannels), nil
	case "priority":
		fallthrough
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

	return s.orderChannels(modelName, healthyChannels), nil
}

func selectWeighted(channels []model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}

	totalWeight := 0
	for _, ch := range channels {
		if ch.Weight > 0 {
			totalWeight += ch.Weight
		}
	}

	if totalWeight <= 0 {
		return &channels[0]
	}

	pick := rand.Intn(totalWeight)
	current := 0
	for i := range channels {
		weight := channels[i].Weight
		if weight <= 0 {
			continue
		}
		current += weight
		if pick < current {
			return &channels[i]
		}
	}

	return &channels[0]
}

func (s *Scheduler) selectRoundRobin(modelName string, channels []model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.rrCounters[modelName] % len(channels)
	s.rrCounters[modelName] = index + 1
	return &channels[index]
}

func (s *Scheduler) orderChannels(modelName string, channels []model.Channel) []model.Channel {
	if len(channels) <= 1 {
		return channels
	}

	var selected *model.Channel
	switch s.strategy {
	case "weighted":
		selected = selectWeighted(channels)
	case "round_robin":
		selected = s.selectRoundRobin(modelName, channels)
	default:
		selected = &channels[0]
	}

	if selected == nil {
		return channels
	}

	ordered := make([]model.Channel, 0, len(channels))
	ordered = append(ordered, *selected)
	for _, ch := range channels {
		if ch.ID != selected.ID {
			ordered = append(ordered, ch)
		}
	}

	return ordered
}
