package repository

import (
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type SystemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository(db *gorm.DB) *SystemConfigRepository {
	return &SystemConfigRepository{db: db}
}

// Get 获取系统配置值。
func (r *SystemConfigRepository) Get(key string) (string, error) {
	var config model.SystemConfig
	err := r.db.Where("key = ?", key).First(&config).Error
	if err != nil {
		return "", err
	}
	return config.Value, nil
}

// Set 写入系统配置值。
func (r *SystemConfigRepository) Set(key, value string) error {
	config := model.SystemConfig{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	}
	return r.db.Save(&config).Error
}
