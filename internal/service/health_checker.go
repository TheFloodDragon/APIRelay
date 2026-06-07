package service

import (
	"context"
	"log"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/adapter"
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

// HealthChecker 健康检查服务
type HealthChecker struct {
	channelRepo *repository.ChannelRepository
	interval    time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewHealthChecker 创建健康检查服务
func NewHealthChecker(channelRepo *repository.ChannelRepository, intervalSeconds int) *HealthChecker {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		channelRepo: channelRepo,
		interval:    time.Duration(intervalSeconds) * time.Second,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动定时健康检查
func (h *HealthChecker) Start() {
	log.Printf("健康检查服务已启动，检查间隔: %v", h.interval)

	ticker := time.NewTicker(h.interval)
	go func() {
		defer ticker.Stop()

		// 立即执行一次，但不阻塞 HTTP 服务启动。
		h.checkAll()

		for {
			select {
			case <-ticker.C:
				h.checkAll()
			case <-h.ctx.Done():
				log.Println("健康检查服务已停止")
				return
			}
		}
	}()
}

// Stop 停止健康检查
func (h *HealthChecker) Stop() {
	h.cancel()
}

func (h *HealthChecker) checkAll() {
	channels, err := h.channelRepo.GetAll()
	if err != nil {
		log.Printf("健康检查: 获取渠道列表失败: %v", err)
		return
	}

	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}

		status := h.checkChannel(&channel)
		if err := h.channelRepo.UpdateHealthStatus(channel.ID, status); err != nil {
			log.Printf("健康检查: 更新渠道 %s 状态失败: %v", channel.Name, err)
		}
	}
}

func (h *HealthChecker) checkChannel(channel *model.Channel) string {
	fetcher := adapter.GetModelFetcher(channel.Type, channel.APIKey, channel.BaseURL)
	if fetcher == nil {
		return "unknown"
	}

	_, err := fetcher.FetchModels()
	if err != nil {
		log.Printf("健康检查: 渠道 %s 检查失败: %v", channel.Name, err)
		return "unhealthy"
	}

	log.Printf("健康检查: 渠道 %s 状态正常", channel.Name)
	return "healthy"
}
