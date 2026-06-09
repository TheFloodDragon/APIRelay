package handler

import (
	"net/http"
	"strconv"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/gin-gonic/gin"
)

type ProxyConfigStore interface {
	GetProxyConfig() (*model.ProxyConfig, error)
	SaveProxyConfig(config *model.ProxyConfig) error
}

type FailoverQueueStore interface {
	GetFailoverQueue() ([]model.FailoverQueueItem, error)
	SaveFailoverQueue(channelIDs []uint) error
}

type ChannelStore interface {
	GetAll() ([]model.Channel, error)
}

type ProviderHealthStore interface {
	GetProviderHealth(channelID uint) (*model.ProviderHealth, error)
}

type CircuitManager interface {
	CircuitSnapshots(channelIDs []uint) []circuit.Snapshot
	ResetCircuit(channelID uint)
	ApplyProxyConfig(config *model.ProxyConfig)
}

type ProxyHandler struct {
	configRepo    ProxyConfigStore
	queueRepo     FailoverQueueStore
	channelRepo   ChannelStore
	healthRepo    ProviderHealthStore
	circuitRouter CircuitManager
}

type proxyConfigRequest struct {
	Enabled                   *bool `json:"enabled"`
	AutoFailoverEnabled       *bool `json:"auto_failover_enabled"`
	MaxRetries                *int  `json:"max_retries"`
	NonStreamingTimeoutMS     *int  `json:"non_streaming_timeout_ms"`
	StreamingFirstByteTimeout *int  `json:"streaming_first_byte_timeout"`
	StreamingIdleTimeoutMS    *int  `json:"streaming_idle_timeout_ms"`
	CircuitFailureThreshold   *int  `json:"circuit_failure_threshold"`
	CircuitSuccessThreshold   *int  `json:"circuit_success_threshold"`
	CircuitOpenSeconds        *int  `json:"circuit_open_seconds"`
}

type saveFailoverQueueRequest struct {
	ChannelIDs []uint `json:"channel_ids" binding:"required"`
}

type circuitStatus struct {
	ChannelID uint                  `json:"channel_id"`
	Channel   *safeChannel          `json:"channel,omitempty"`
	Health    *model.ProviderHealth `json:"health,omitempty"`
	Circuit   circuit.Snapshot      `json:"circuit"`
}

type safeChannel struct {
	ID           uint                 `json:"id"`
	Name         string               `json:"name"`
	Type         string               `json:"type"`
	BaseURL      string               `json:"base_url"`
	Models       model.JSONStringList `json:"models"`
	Priority     int                  `json:"priority"`
	Weight       int                  `json:"weight"`
	Enabled      bool                 `json:"enabled"`
	Timeout      int                  `json:"timeout"`
	MaxRetries   int                  `json:"max_retries"`
	Config       model.JSONMap        `json:"config"`
	HealthStatus string               `json:"health_status"`
	LastCheck    any                  `json:"last_check,omitempty"`
}

func NewProxyHandler(
	configRepo *repository.ProxyConfigRepository,
	queueRepo *repository.FailoverQueueRepository,
	channelRepo *repository.ChannelRepository,
	healthRepo *repository.ProviderHealthRepository,
	circuitRouter CircuitManager,
) *ProxyHandler {
	return &ProxyHandler{
		configRepo:    configRepo,
		queueRepo:     queueRepo,
		channelRepo:   channelRepo,
		healthRepo:    healthRepo,
		circuitRouter: circuitRouter,
	}
}

func NewProxyHandlerWithStores(
	configRepo ProxyConfigStore,
	queueRepo FailoverQueueStore,
	channelRepo ChannelStore,
	healthRepo ProviderHealthStore,
	circuitRouter CircuitManager,
) *ProxyHandler {
	return &ProxyHandler{
		configRepo:    configRepo,
		queueRepo:     queueRepo,
		channelRepo:   channelRepo,
		healthRepo:    healthRepo,
		circuitRouter: circuitRouter,
	}
}

func (h *ProxyHandler) GetStatus(c *gin.Context) {
	config, err := h.loadConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理配置失败: " + err.Error()})
		return
	}
	queue, err := h.loadQueue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取故障转移队列失败: " + err.Error()})
		return
	}
	circuits, err := h.buildCircuitStatuses()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取熔断器状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"config":         config,
			"failover_queue": queue,
			"circuits":       circuits,
		},
	})
}

func (h *ProxyHandler) GetConfig(c *gin.Context) {
	config, err := h.loadConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *ProxyHandler) UpdateConfig(c *gin.Context) {
	var req proxyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	config, err := h.loadConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取代理配置失败: " + err.Error()})
		return
	}
	applyProxyConfigRequest(config, req)
	normalizeProxyConfig(config)

	if h.configRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "代理配置仓库未初始化"})
		return
	}
	if err := h.configRepo.SaveProxyConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存代理配置失败: " + err.Error()})
		return
	}
	if h.circuitRouter != nil {
		h.circuitRouter.ApplyProxyConfig(config)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

func (h *ProxyHandler) GetFailoverQueue(c *gin.Context) {
	queue, err := h.loadQueue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取故障转移队列失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

func (h *ProxyHandler) UpdateFailoverQueue(c *gin.Context) {
	var req saveFailoverQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}
	channelIDs := normalizeChannelIDs(req.ChannelIDs)
	if h.queueRepo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "故障转移队列仓库未初始化"})
		return
	}
	if err := h.queueRepo.SaveFailoverQueue(channelIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存故障转移队列失败: " + err.Error()})
		return
	}

	queue, err := h.loadQueue()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取故障转移队列失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": queue})
}

func (h *ProxyHandler) GetCircuits(c *gin.Context) {
	circuits, err := h.buildCircuitStatuses()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取熔断器状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": circuits})
}

func (h *ProxyHandler) ResetCircuit(c *gin.Context) {
	channelID, err := strconv.ParseUint(c.Param("channel_id"), 10, 32)
	if err != nil || channelID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的渠道ID"})
		return
	}
	if h.circuitRouter != nil {
		h.circuitRouter.ResetCircuit(uint(channelID))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "熔断器已重置"})
}

func (h *ProxyHandler) loadConfig() (*model.ProxyConfig, error) {
	if h.configRepo == nil {
		config := repository.DefaultProxyConfig()
		return &config, nil
	}
	config, err := h.configRepo.GetProxyConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		defaultConfig := repository.DefaultProxyConfig()
		config = &defaultConfig
	}
	return config, nil
}

func (h *ProxyHandler) loadQueue() ([]model.FailoverQueueItem, error) {
	if h.queueRepo == nil {
		return []model.FailoverQueueItem{}, nil
	}
	queue, err := h.queueRepo.GetFailoverQueue()
	if err != nil {
		return nil, err
	}
	sanitizeQueue(queue)
	return queue, nil
}

func (h *ProxyHandler) buildCircuitStatuses() ([]circuitStatus, error) {
	channels := []model.Channel{}
	if h.channelRepo != nil {
		loaded, err := h.channelRepo.GetAll()
		if err != nil {
			return nil, err
		}
		channels = loaded
	}

	ids := make([]uint, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}
	var snapshots []circuit.Snapshot
	if h.circuitRouter != nil {
		snapshots = h.circuitRouter.CircuitSnapshots(ids)
	}
	if len(snapshots) == 0 && len(ids) > 0 {
		snapshots = make([]circuit.Snapshot, 0, len(ids))
		for _, id := range ids {
			snapshots = append(snapshots, circuit.Snapshot{ChannelID: id, State: circuit.CircuitStateClosed})
		}
	}

	channelsByID := make(map[uint]model.Channel, len(channels))
	for _, channel := range channels {
		channelsByID[channel.ID] = channel
	}
	statuses := make([]circuitStatus, 0, len(snapshots))
	for _, snapshot := range snapshots {
		status := circuitStatus{
			ChannelID: snapshot.ChannelID,
			Circuit:   snapshot,
		}
		if channel, ok := channelsByID[snapshot.ChannelID]; ok {
			status.Channel = safeChannelFromModel(channel)
		}
		if h.healthRepo != nil && snapshot.ChannelID != 0 {
			if health, err := h.healthRepo.GetProviderHealth(snapshot.ChannelID); err == nil {
				status.Health = health
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func applyProxyConfigRequest(config *model.ProxyConfig, req proxyConfigRequest) {
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.AutoFailoverEnabled != nil {
		config.AutoFailoverEnabled = *req.AutoFailoverEnabled
	}
	if req.MaxRetries != nil {
		config.MaxRetries = *req.MaxRetries
	}
	if req.NonStreamingTimeoutMS != nil {
		config.NonStreamingTimeoutMS = *req.NonStreamingTimeoutMS
	}
	if req.StreamingFirstByteTimeout != nil {
		config.StreamingFirstByteTimeout = *req.StreamingFirstByteTimeout
	}
	if req.StreamingIdleTimeoutMS != nil {
		config.StreamingIdleTimeoutMS = *req.StreamingIdleTimeoutMS
	}
	if req.CircuitFailureThreshold != nil {
		config.CircuitFailureThreshold = *req.CircuitFailureThreshold
	}
	if req.CircuitSuccessThreshold != nil {
		config.CircuitSuccessThreshold = *req.CircuitSuccessThreshold
	}
	if req.CircuitOpenSeconds != nil {
		config.CircuitOpenSeconds = *req.CircuitOpenSeconds
	}
}

func normalizeProxyConfig(config *model.ProxyConfig) {
	defaults := repository.DefaultProxyConfig()
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.NonStreamingTimeoutMS <= 0 {
		config.NonStreamingTimeoutMS = defaults.NonStreamingTimeoutMS
	}
	if config.StreamingFirstByteTimeout <= 0 {
		config.StreamingFirstByteTimeout = defaults.StreamingFirstByteTimeout
	}
	if config.StreamingIdleTimeoutMS <= 0 {
		config.StreamingIdleTimeoutMS = defaults.StreamingIdleTimeoutMS
	}
	if config.CircuitFailureThreshold <= 0 {
		config.CircuitFailureThreshold = defaults.CircuitFailureThreshold
	}
	if config.CircuitSuccessThreshold <= 0 {
		config.CircuitSuccessThreshold = defaults.CircuitSuccessThreshold
	}
	if config.CircuitOpenSeconds <= 0 {
		config.CircuitOpenSeconds = defaults.CircuitOpenSeconds
	}
}

func normalizeChannelIDs(channelIDs []uint) []uint {
	seen := make(map[uint]struct{}, len(channelIDs))
	result := make([]uint, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID == 0 {
			continue
		}
		if _, exists := seen[channelID]; exists {
			continue
		}
		seen[channelID] = struct{}{}
		result = append(result, channelID)
	}
	return result
}

func sanitizeQueue(queue []model.FailoverQueueItem) {
	for index := range queue {
		if queue[index].Channel != nil {
			queue[index].Channel.APIKey = ""
		}
	}
}

func safeChannelFromModel(channel model.Channel) *safeChannel {
	return &safeChannel{
		ID:           channel.ID,
		Name:         channel.Name,
		Type:         channel.Type,
		BaseURL:      channel.BaseURL,
		Models:       channel.Models,
		Priority:     channel.Priority,
		Weight:       channel.Weight,
		Enabled:      channel.Enabled,
		Timeout:      channel.Timeout,
		MaxRetries:   channel.MaxRetries,
		Config:       channel.Config,
		HealthStatus: channel.HealthStatus,
		LastCheck:    channel.LastCheck,
	}
}
