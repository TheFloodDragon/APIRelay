package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

	models, err := fetchModels(channel)
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

func fetchModels(channel *model.Channel) ([]string, error) {
	if channel == nil {
		return nil, fmt.Errorf("channel is nil")
	}

	switch strings.ToLower(strings.TrimSpace(channel.Type)) {
	case "anthropic", "claude":
		return []string{
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
			"claude-2.1",
			"claude-2.0",
		}, nil
	case "gemini", "google":
		return []string{
			"gemini-2.0-flash-exp",
			"gemini-1.5-pro",
			"gemini-1.5-flash",
			"gemini-1.0-pro",
		}, nil
	default:
		return fetchOpenAICompatibleModels(channel)
	}
}

func fetchOpenAICompatibleModels(channel *model.Channel) ([]string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if channel.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+channel.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API返回错误: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	return models, nil
}
