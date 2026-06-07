package repository

import (
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type ModelRepository struct {
	db *gorm.DB
}

func NewModelRepository(db *gorm.DB) *ModelRepository {
	return &ModelRepository{db: db}
}

// GetAll 获取所有模型
func (r *ModelRepository) GetAll() ([]model.Model, error) {
	var models []model.Model
	err := r.db.Preload("Channel").Find(&models).Error
	return models, err
}

// GetEnabled 获取所有启用的模型
func (r *ModelRepository) GetEnabled() ([]model.Model, error) {
	var models []model.Model
	err := r.db.Where("enabled = ?", true).Preload("Channel").Find(&models).Error
	return models, err
}

// GetByName 根据名称获取模型
func (r *ModelRepository) GetByName(name string) (*model.Model, error) {
	var m model.Model
	err := r.db.Where("name = ?", name).Preload("Channel").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByChannelID 获取指定渠道的所有模型
func (r *ModelRepository) GetByChannelID(channelID uint) ([]model.Model, error) {
	var models []model.Model
	err := r.db.Where("channel_id = ?", channelID).Find(&models).Error
	return models, err
}

// Create 创建模型
func (r *ModelRepository) Create(m *model.Model) error {
	return r.db.Create(m).Error
}

// CreateBatch 批量创建模型
func (r *ModelRepository) CreateBatch(models []model.Model) error {
	return r.db.Create(&models).Error
}

// Update 更新模型
func (r *ModelRepository) Update(m *model.Model) error {
	return r.db.Save(m).Error
}

// Delete 删除模型
func (r *ModelRepository) Delete(id uint) error {
	return r.db.Delete(&model.Model{}, id).Error
}

// DeleteByChannelID 删除指定渠道的所有模型
func (r *ModelRepository) DeleteByChannelID(channelID uint) error {
	return r.db.Where("channel_id = ?", channelID).Delete(&model.Model{}).Error
}
