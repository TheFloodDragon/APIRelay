package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/adapter"
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

// HealthChecker 健康检查服务
type HealthChecker struct {
	channelRepo        *repository.ChannelRepository
	interval           time.Duration
	unhealthyThreshold int
	failureCounts      map[uint]int
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
}

// NewHealthChecker 创建健康检查服务
func NewHealthChecker(channelRepo *repository.ChannelRepository, intervalSeconds, unhealthyThreshold int) *HealthChecker {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	if unhealthyThreshold <= 0 {
		unhealthyThreshold = 3
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		channelRepo:        channelRepo,
		interval:           time.Duration(intervalSeconds) * time.Second,
		unhealthyThreshold: unhealthyThreshold,
		failureCounts:      make(map[uint]int),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start 启动定时健康检查
func (h *HealthChecker) Start() {
	log.Printf("健康检查服务已启动，检查间隔: %v，失败阈值: %d", h.interval, h.unhealthyThreshold)

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

	seenEnabled := make(map[uint]struct{}, len(channels))
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		seenEnabled[channel.ID] = struct{}{}

		status := h.statusAfterCheck(&channel)
		if err := h.channelRepo.UpdateHealthStatus(channel.ID, status); err != nil {
			log.Printf("健康检查: 更新渠道 %s 状态失败: %v", channel.Name, err)
		}
	}
	h.pruneFailureCounts(seenEnabled)
}

func (h *HealthChecker) statusAfterCheck(channel *model.Channel) string {
	if err := h.checkChannel(channel); err != nil {
		failures := h.recordFailure(channel.ID)
		if failures >= h.unhealthyThreshold {
			log.Printf("健康检查: 渠道 %s 连续失败 %d 次，标记为 unhealthy: %v", channel.Name, failures, err)
			return "unhealthy"
		}

		log.Printf("健康检查: 渠道 %s 检查失败 (%d/%d): %v", channel.Name, failures, h.unhealthyThreshold, err)
		return nonEmptyStatus(channel.HealthStatus)
	}

	h.resetFailure(channel.ID)
	log.Printf("健康检查: 渠道 %s 状态正常", channel.Name)
	return "healthy"
}

func (h *HealthChecker) checkChannel(channel *model.Channel) error {
	fetcher := adapter.GetModelFetcher(channel.Type, channel.APIKey, channel.BaseURL)
	if fetcher == nil {
		return nil
	}

	_, err := fetcher.FetchModels()
	return err
}

func (h *HealthChecker) recordFailure(channelID uint) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failureCounts[channelID]++
	return h.failureCounts[channelID]
}

func (h *HealthChecker) resetFailure(channelID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.failureCounts, channelID)
}

func (h *HealthChecker) pruneFailureCounts(enabledIDs map[uint]struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for channelID := range h.failureCounts {
		if _, ok := enabledIDs[channelID]; !ok {
			delete(h.failureCounts, channelID)
		}
	}
}

func nonEmptyStatus(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}
