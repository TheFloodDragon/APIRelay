package controller

import "github.com/TheFloodDragon/APIRelay/internal/model"

type staticChannelRepo struct {
	channels []model.Channel
}

func (r *staticChannelRepo) GetEnabled() ([]model.Channel, error) {
	return r.channels, nil
}

type staticProxyConfigRepo struct {
	maxRetries int
}

func (r *staticProxyConfigRepo) GetProxyConfig() (*model.ProxyConfig, error) {
	maxRetries := 0
	if r != nil && r.maxRetries > 0 {
		maxRetries = r.maxRetries
	}
	return &model.ProxyConfig{
		Enabled:             true,
		AutoFailoverEnabled: true,
		MaxRetries:          maxRetries,
	}, nil
}

type staticFailoverQueueRepo struct {
	items []model.FailoverQueueItem
}

func (r *staticFailoverQueueRepo) GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	return r.items, nil
}

type staticProviderHealthRepo struct{}

func (r *staticProviderHealthRepo) GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	return &model.ProviderHealth{ChannelID: channelID, IsHealthy: true}, nil
}

func (r *staticProviderHealthRepo) UpdateProviderHealth(_ *model.ProviderHealth) error {
	return nil
}
