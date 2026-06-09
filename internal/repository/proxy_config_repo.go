package repository

import (
	"errors"
	"fmt"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type ProxyConfigRepository struct {
	db *gorm.DB
}

func GetProxyConfig() (*model.ProxyConfig, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return NewProxyConfigRepository(model.DB).GetProxyConfig()
}

func SaveProxyConfig(config *model.ProxyConfig) error {
	if model.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return NewProxyConfigRepository(model.DB).SaveProxyConfig(config)
}

func NewProxyConfigRepository(db *gorm.DB) *ProxyConfigRepository {
	return &ProxyConfigRepository{db: db}
}

func (r *ProxyConfigRepository) GetProxyConfig() (*model.ProxyConfig, error) {
	var config model.ProxyConfig
	err := r.db.First(&config, 1).Error
	if err == nil {
		return &config, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	config = DefaultProxyConfig()
	if err := r.db.Create(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ProxyConfigRepository) SaveProxyConfig(config *model.ProxyConfig) error {
	if config == nil {
		return nil
	}
	config.ID = 1
	return r.db.Save(config).Error
}

func DefaultProxyConfig() model.ProxyConfig {
	return model.ProxyConfig{
		ID:                        1,
		Enabled:                   true,
		AutoFailoverEnabled:       true,
		MaxRetries:                2,
		NonStreamingTimeoutMS:     60000,
		StreamingFirstByteTimeout: 5000,
		StreamingIdleTimeoutMS:    60000,
		CircuitFailureThreshold:   3,
		CircuitSuccessThreshold:   1,
		CircuitOpenSeconds:        30,
	}
}
