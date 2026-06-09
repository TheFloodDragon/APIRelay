package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type ProviderHealthRepository struct {
	db *gorm.DB
}

func GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return NewProviderHealthRepository(model.DB).GetProviderHealth(channelID)
}

func UpdateProviderHealth(health *model.ProviderHealth) error {
	if model.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return NewProviderHealthRepository(model.DB).UpdateProviderHealth(health)
}

func NewProviderHealthRepository(db *gorm.DB) *ProviderHealthRepository {
	return &ProviderHealthRepository{db: db}
}

func (r *ProviderHealthRepository) GetProviderHealth(channelID uint) (*model.ProviderHealth, error) {
	var health model.ProviderHealth
	err := r.db.First(&health, "channel_id = ?", channelID).Error
	if err == nil {
		return &health, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	health = model.ProviderHealth{
		ChannelID: channelID,
		IsHealthy: true,
		UpdatedAt: now,
	}
	if err := r.db.Create(&health).Error; err != nil {
		return nil, err
	}
	return &health, nil
}

func (r *ProviderHealthRepository) UpdateProviderHealth(health *model.ProviderHealth) error {
	if health == nil {
		return nil
	}
	health.UpdatedAt = time.Now()
	return r.db.Save(health).Error
}
