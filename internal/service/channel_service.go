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

	return s.channelRepo.Create(channel)
}

// UpdateChannel 更新渠道
func (s *ChannelService) UpdateChannel(channel *model.Channel) error {
	return s.channelRepo.Update(channel)
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

	// 同步到模型表
	if err := s.modelRepo.DeleteByChannelID(channelID); err != nil {
		return nil, err
	}

	modelRecords := make([]model.Model, 0, len(models))
	for _, name := range models {
		modelRecords = append(modelRecords, model.Model{
			Name:      name,
			ChannelID: channelID,
			Enabled:   true,
		})
	}

	if err := s.modelRepo.CreateBatch(modelRecords); err != nil {
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
