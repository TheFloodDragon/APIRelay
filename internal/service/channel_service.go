package service

import (
	"fmt"

	"github.com/TheFloodDragon/APIRelay/internal/adapter"
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

type ChannelService struct {
	channelRepo *repository.ChannelRepository
	modelRepo   *repository.ModelRepository
}

func NewChannelService(channelRepo *repository.ChannelRepository, modelRepo *repository.ModelRepository) *ChannelService {
	return &ChannelService{
		channelRepo: channelRepo,
		modelRepo:   modelRepo,
	}
}

// GetAllChannels 获取所有渠道
func (s *ChannelService) GetAllChannels() ([]model.Channel, error) {
	return s.channelRepo.GetAll()
}

// GetChannel 获取单个渠道
func (s *ChannelService) GetChannel(id uint) (*model.Channel, error) {
	return s.channelRepo.GetByID(id)
}

// CreateChannel 创建渠道
func (s *ChannelService) CreateChannel(channel *model.Channel) error {
	if channel.Type == "" {
		channel.Type = "openai_compatible"
	}
	if channel.Timeout == 0 {
		channel.Timeout = 60000
	}
	if channel.MaxRetries == 0 {
		channel.MaxRetries = 3
	}
	if channel.Weight == 0 {
		channel.Weight = 1
	}
	if channel.HealthStatus == "" {
		channel.HealthStatus = "unknown"
	}

	if err := s.channelRepo.Create(channel); err != nil {
		return err
	}

	// 同步手动填写的模型列表到模型表
	if len(channel.Models) > 0 {
		return s.modelRepo.SyncChannelModels(channel.ID, channel.Models)
	}
	return nil
}

// UpdateChannel 更新渠道
func (s *ChannelService) UpdateChannel(channel *model.Channel) error {
	if err := s.channelRepo.Update(channel); err != nil {
		return err
	}

	// 同步手动填写的模型列表到模型表
	if len(channel.Models) > 0 {
		return s.modelRepo.SyncChannelModels(channel.ID, channel.Models)
	}
	return nil
}

// DeleteChannel 删除渠道
func (s *ChannelService) DeleteChannel(id uint) error {
	// 先删除关联的模型
	if err := s.modelRepo.DeleteByChannelID(id); err != nil {
		return err
	}
	return s.channelRepo.Delete(id)
}

// ReorderChannels 批量更新渠道优先级
type ReorderItem struct {
	ID       uint `json:"id" binding:"required"`
	Priority int  `json:"priority" binding:"required"`
}

func (s *ChannelService) ReorderChannels(orders []ReorderItem) error {
	for _, order := range orders {
		if err := s.channelRepo.UpdatePriority(order.ID, order.Priority); err != nil {
			return fmt.Errorf("更新渠道 %d 优先级失败: %w", order.ID, err)
		}
	}
	return nil
}

// FetchModels 获取并同步模型列表
func (s *ChannelService) FetchModels(channelID uint) ([]string, error) {
	channel, err := s.channelRepo.GetByID(channelID)
	if err != nil {
		return nil, err
	}

	fetcher := adapter.GetModelFetcher(channel.Type, channel.APIKey, channel.BaseURL)
	models, err := fetcher.FetchModels()
	if err != nil {
		return nil, err
	}

	// 更新渠道模型列表
	channel.Models = model.JSONStringList(models)
	if err := s.channelRepo.Update(channel); err != nil {
		return nil, err
	}

	// 使用新的同步方法，保留现有模型的元数据
	if err := s.modelRepo.SyncChannelModels(channelID, models); err != nil {
		return nil, err
	}

	return models, nil
}

// TestChannel 测试渠道连接
func (s *ChannelService) TestChannel(channelID uint) error {
	channel, err := s.channelRepo.GetByID(channelID)
	if err != nil {
		return err
	}

	fetcher := adapter.GetModelFetcher(channel.Type, channel.APIKey, channel.BaseURL)
	_, err = fetcher.FetchModels()
	if err != nil {
		_ = s.channelRepo.UpdateHealthStatus(channelID, "unhealthy")
		return err
	}

	_ = s.channelRepo.UpdateHealthStatus(channelID, "healthy")
	return nil
}
