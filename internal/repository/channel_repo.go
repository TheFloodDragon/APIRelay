package repository

import (
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type ChannelRepository struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

// GetAll 获取所有渠道，按优先级排序
func (r *ChannelRepository) GetAll() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.db.Order("priority DESC, id ASC").Find(&channels).Error
	return channels, err
}

// GetByID 根据ID获取渠道
func (r *ChannelRepository) GetByID(id uint) (*model.Channel, error) {
	var channel model.Channel
	err := r.db.First(&channel, id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetEnabled 获取所有启用的渠道
func (r *ChannelRepository) GetEnabled() ([]model.Channel, error) {
	var channels []model.Channel
	err := r.db.Where("enabled = ?", true).Order("priority DESC, id ASC").Find(&channels).Error
	return channels, err
}

// GetByModel 根据模型名获取支持该模型的渠道
func (r *ChannelRepository) GetByModel(modelName string) ([]model.Channel, error) {
	var channels []model.Channel
	err := r.db.Where("enabled = ? AND models LIKE ?", true, "%\""+modelName+"\"%").
		Order("priority DESC, weight DESC, id ASC").
		Find(&channels).Error
	return channels, err
}

// Create 创建渠道
func (r *ChannelRepository) Create(channel *model.Channel) error {
	return r.db.Create(channel).Error
}

// Update 更新渠道
func (r *ChannelRepository) Update(channel *model.Channel) error {
	return r.db.Save(channel).Error
}

// Delete 删除渠道
func (r *ChannelRepository) Delete(id uint) error {
	return r.db.Delete(&model.Channel{}, id).Error
}

// UpdatePriority 更新渠道优先级
func (r *ChannelRepository) UpdatePriority(id uint, priority int) error {
	return r.db.Model(&model.Channel{}).Where("id = ?", id).Update("priority", priority).Error
}

// UpdateHealthStatus 更新健康状态并记录检查时间
func (r *ChannelRepository) UpdateHealthStatus(id uint, status string) error {
	return r.UpdateHealthCheck(id, status, nil)
}

// UpdateHealthCheck 更新健康状态和检查时间。lastCheck 为空时使用数据库当前时间。
func (r *ChannelRepository) UpdateHealthCheck(id uint, status string, lastCheck interface{}) error {
	if lastCheck == nil {
		lastCheck = gorm.Expr("CURRENT_TIMESTAMP")
	}
	return r.db.Model(&model.Channel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"health_status": status,
		"last_check":    lastCheck,
	}).Error
}
