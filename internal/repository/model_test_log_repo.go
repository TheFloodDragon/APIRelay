package repository

import (
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type ModelTestLogRepository struct {
	db *gorm.DB
}

func NewModelTestLogRepository(db *gorm.DB) *ModelTestLogRepository {
	return &ModelTestLogRepository{db: db}
}

// Create 创建模型测试日志。
func (r *ModelTestLogRepository) Create(log *model.ModelTestLog) error {
	return r.db.Create(log).Error
}

// GetByChannel 获取指定渠道最近的模型测试日志。
func (r *ModelTestLogRepository) GetByChannel(channelID uint, limit int) ([]model.ModelTestLog, error) {
	if limit <= 0 {
		limit = 20
	}
	var logs []model.ModelTestLog
	err := r.db.Where("channel_id = ?", channelID).
		Preload("Channel").
		Order("tested_at DESC, id DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
