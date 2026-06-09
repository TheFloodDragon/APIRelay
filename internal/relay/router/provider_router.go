package router

import (
	"sort"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/relay/circuit"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
)

type ProviderRouter struct {
	channels       ChannelRepo
	proxyConfig    ProxyConfigRepo
	failoverQueue  FailoverQueueRepo
	providerHealth ProviderHealthRepo
	breaker        *circuit.Breaker
}

func NewProviderRouter(
	channels ChannelRepo,
	proxyConfig ProxyConfigRepo,
	failoverQueue FailoverQueueRepo,
	providerHealth ProviderHealthRepo,
	breaker *circuit.Breaker,
) *ProviderRouter {
	return &ProviderRouter{
		channels:       channels,
		proxyConfig:    proxyConfig,
		failoverQueue:  failoverQueue,
		providerHealth: providerHealth,
		breaker:        breaker,
	}
}

func NewDefaultProviderRouter(
	channels *repository.ChannelRepository,
	proxyConfig *repository.ProxyConfigRepository,
	failoverQueue *repository.FailoverQueueRepository,
	providerHealth *repository.ProviderHealthRepository,
) *ProviderRouter {
	config, _ := proxyConfig.GetProxyConfig()
	failureThreshold := 3
	successThreshold := 1
	openDuration := 30 * time.Second
	if config != nil {
		failureThreshold = config.CircuitFailureThreshold
		successThreshold = config.CircuitSuccessThreshold
		if config.CircuitOpenSeconds > 0 {
			openDuration = time.Duration(config.CircuitOpenSeconds) * time.Second
		}
	}
	return NewProviderRouter(
		channels,
		proxyConfig,
		failoverQueue,
		providerHealth,
		circuit.NewBreaker(failureThreshold, successThreshold, openDuration),
	)
}

func (r *ProviderRouter) GetProxyConfig() (*model.ProxyConfig, error) {
	if r == nil || r.proxyConfig == nil {
		return nil, nil
	}
	return r.proxyConfig.GetProxyConfig()
}

func (r *ProviderRouter) SelectProviders() ([]model.Channel, error) {
	config, err := r.proxyConfig.GetProxyConfig()
	if err != nil {
		return nil, err
	}
	r.ApplyProxyConfig(config)
	if !config.Enabled {
		return []model.Channel{}, nil
	}

	channels, err := r.channels.GetEnabled()
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return channels, nil
	}

	ordered := channels
	if config.AutoFailoverEnabled {
		ordered, err = r.providersByFailoverQueue(channels)
		if err != nil {
			return nil, err
		}
	} else {
		ordered = providersByPriority(channels)[:1]
	}

	return r.filterCircuitOpen(ordered), nil
}

func (r *ProviderRouter) Allow(channelID uint) circuit.AllowResult {
	if r == nil || r.breaker == nil {
		return circuit.AllowResult{Allowed: true, State: circuit.CircuitStateClosed}
	}
	return r.breaker.Allow(channelID)
}

func (r *ProviderRouter) RecordSuccess(channelID uint) {
	r.RecordSuccessWithPermit(channelID, false)
}

func (r *ProviderRouter) RecordSuccessWithPermit(channelID uint, usedHalfOpenPermit bool) {
	if r == nil || channelID == 0 {
		return
	}
	if r.breaker != nil {
		r.breaker.RecordSuccess(channelID, usedHalfOpenPermit)
	}
	if r.providerHealth != nil {
		now := time.Now()
		health, err := r.providerHealth.GetProviderHealth(channelID)
		if err == nil {
			health.IsHealthy = true
			health.ConsecutiveFailures = 0
			health.LastSuccessAt = &now
			health.LastError = ""
			_ = r.providerHealth.UpdateProviderHealth(health)
		}
	}
}

func (r *ProviderRouter) RecordFailure(channelID uint, errMsg string) {
	r.RecordFailureWithPermit(channelID, errMsg, false)
}

func (r *ProviderRouter) RecordFailureWithPermit(channelID uint, errMsg string, usedHalfOpenPermit bool) {
	if r == nil || channelID == 0 {
		return
	}
	if r.breaker != nil {
		r.breaker.RecordFailure(channelID, usedHalfOpenPermit)
	}
	if r.providerHealth != nil {
		now := time.Now()
		health, err := r.providerHealth.GetProviderHealth(channelID)
		if err == nil {
			health.IsHealthy = false
			health.ConsecutiveFailures++
			health.LastFailureAt = &now
			health.LastError = errMsg
			_ = r.providerHealth.UpdateProviderHealth(health)
		}
	}
}

func (r *ProviderRouter) ResetCircuit(channelID uint) {
	if r == nil || r.breaker == nil {
		return
	}
	r.breaker.Reset(channelID)
}

func (r *ProviderRouter) CircuitSnapshots(channelIDs []uint) []circuit.Snapshot {
	if r == nil || r.breaker == nil {
		return nil
	}
	return r.breaker.Snapshot(channelIDs)
}

func (r *ProviderRouter) ApplyProxyConfig(config *model.ProxyConfig) {
	if r == nil || config == nil {
		return
	}
	openDuration := time.Duration(config.CircuitOpenSeconds) * time.Second
	if r.breaker == nil {
		r.breaker = circuit.NewBreaker(config.CircuitFailureThreshold, config.CircuitSuccessThreshold, openDuration)
		return
	}
	r.breaker.Reconfigure(config.CircuitFailureThreshold, config.CircuitSuccessThreshold, openDuration)
}

func providersByPriority(channels []model.Channel) []model.Channel {
	ordered := append([]model.Channel(nil), channels...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Priority > ordered[j].Priority
	})
	return ordered
}

func (r *ProviderRouter) providersByFailoverQueue(channels []model.Channel) ([]model.Channel, error) {
	items, err := r.failoverQueue.GetFailoverQueue()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return channels, nil
	}

	byID := make(map[uint]model.Channel, len(channels))
	for _, channel := range channels {
		byID[channel.ID] = channel
	}

	ordered := make([]model.Channel, 0, len(channels))
	seen := make(map[uint]struct{}, len(channels))
	for _, item := range items {
		channel, ok := byID[item.ChannelID]
		if !ok {
			continue
		}
		ordered = append(ordered, channel)
		seen[channel.ID] = struct{}{}
	}
	for _, channel := range channels {
		if _, exists := seen[channel.ID]; exists {
			continue
		}
		ordered = append(ordered, channel)
	}
	return ordered, nil
}

func (r *ProviderRouter) filterCircuitOpen(channels []model.Channel) []model.Channel {
	if r.breaker == nil || len(channels) == 0 {
		return channels
	}
	filtered := make([]model.Channel, 0, len(channels))
	for _, channel := range channels {
		if r.breaker.State(channel.ID) != circuit.CircuitStateOpen {
			filtered = append(filtered, channel)
		}
	}
	return filtered
}
