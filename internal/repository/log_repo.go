package repository

import (
	"time"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db}
}

// Create 创建日志
func (r *LogRepository) Create(log *model.RequestLog) error {
	return r.db.Create(log).Error
}

// GetAll 获取所有日志
func (r *LogRepository) GetAll(limit, offset int) ([]model.RequestLog, error) {
	var logs []model.RequestLog
	err := r.db.Preload("Channel").Preload("APIKey").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs).Error
	return logs, err
}

// GetByChannel 根据渠道获取日志
func (r *LogRepository) GetByChannel(channelID uint, limit, offset int) ([]model.RequestLog, error) {
	var logs []model.RequestLog
	err := r.db.Where("channel_id = ?", channelID).
		Preload("Channel").Preload("APIKey").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs).Error
	return logs, err
}

// GetByTimeRange 根据时间范围获取日志
func (r *LogRepository) GetByTimeRange(start, end time.Time, limit, offset int) ([]model.RequestLog, error) {
	var logs []model.RequestLog
	err := r.db.Where("created_at BETWEEN ? AND ?", start, end).
		Preload("Channel").Preload("APIKey").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs).Error
	return logs, err
}

// Count 统计日志总数
func (r *LogRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.RequestLog{}).Count(&count).Error
	return count, err
}

// CountByChannel 统计指定渠道的日志数
func (r *LogRepository) CountByChannel(channelID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.RequestLog{}).Where("channel_id = ?", channelID).Count(&count).Error
	return count, err
}
