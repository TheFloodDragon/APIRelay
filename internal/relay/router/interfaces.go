package router

import (
	"github.com/TheFloodDragon/APIRelay/internal/model"
)

type ChannelRepo interface {
	GetEnabled() ([]model.Channel, error)
}

type ProxyConfigRepo interface {
	GetProxyConfig() (*model.ProxyConfig, error)
}

type FailoverQueueRepo interface {
	GetFailoverQueue() ([]model.FailoverQueueItem, error)
}

type ProviderHealthRepo interface {
	GetProviderHealth(channelID uint) (*model.ProviderHealth, error)
	UpdateProviderHealth(health *model.ProviderHealth) error
}
