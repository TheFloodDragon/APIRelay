package repository

import (
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// GetByName 根据名称获取模型（查找上游真实模型名）
func (r *ModelRepository) GetByName(name string) (*model.Model, error) {
	var m model.Model
	err := r.db.Where("name = ?", name).Preload("Channel").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByDisplayName 根据显示名称获取启用的模型（用于路由解析）
func (r *ModelRepository) GetByDisplayName(displayName string) ([]model.Model, error) {
	var models []model.Model
	// 优先匹配 display_name，兼容旧数据的 name
	err := r.db.Where("enabled = ? AND (display_name = ? OR (display_name = '' AND name = ?))", true, displayName, displayName).
		Preload("Channel").
		Find(&models).Error
	return models, err
}

// GetByID 根据 ID 获取模型
func (r *ModelRepository) GetByID(id uint) (*model.Model, error) {
	var m model.Model
	err := r.db.Where("id = ?", id).Preload("Channel").First(&m).Error
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

// Create 创建模型，遇到已存在的模型名+渠道时忽略。
func (r *ModelRepository) Create(m *model.Model) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(m).Error
}

// CreateBatch 批量创建模型，遇到已存在的模型名+渠道时忽略。
func (r *ModelRepository) CreateBatch(models []model.Model) error {
	if len(models) == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models).Error
}

// SyncChannelModels 同步指定渠道的模型列表：
// 新增不存在的模型，保留现有模型的元数据（display_name/enabled），删除不在列表中的模型
func (r *ModelRepository) SyncChannelModels(channelID uint, modelNames []string) error {
	if len(modelNames) == 0 {
		// 如果列表为空，删除该渠道的所有模型
		return r.DeleteByChannelID(channelID)
	}

	// 获取该渠道现有模型
	existing, err := r.GetByChannelID(channelID)
	if err != nil {
		return err
	}

	existingMap := make(map[string]*model.Model)
	for i := range existing {
		existingMap[existing[i].Name] = &existing[i]
	}

	// 新增不存在的模型
	var toCreate []model.Model
	for _, name := range modelNames {
		if _, exists := existingMap[name]; !exists {
			toCreate = append(toCreate, model.Model{
				Name:        name,
				DisplayName: name, // 默认显示名与上游名相同
				ChannelID:   channelID,
				Enabled:     true,
				TestEnabled: true,
			})
		}
	}
	if len(toCreate) > 0 {
		if err := r.CreateBatch(toCreate); err != nil {
			return err
		}
	}

	// 删除不在新列表中的模型
	nameSet := make(map[string]struct{})
	for _, name := range modelNames {
		nameSet[name] = struct{}{}
	}
	for _, m := range existing {
		if _, inList := nameSet[m.Name]; !inList {
			if err := r.Delete(m.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetTestCandidatesByDisplayName 获取同一调用名下允许测试的模型记录。
func (r *ModelRepository) GetTestCandidatesByDisplayName(displayName string) ([]model.Model, error) {
	var models []model.Model
	err := r.db.Where("test_enabled = ? AND (display_name = ? OR (display_name = '' AND name = ?))", true, displayName, displayName).
		Preload("Channel").
		Order("id ASC").
		Find(&models).Error
	return models, err
}

// UpdateMetadata 更新模型的元数据（display_name, enabled, test_enabled）
func (r *ModelRepository) UpdateMetadata(id uint, displayName string, enabled bool, testEnabled bool) error {
	return r.db.Model(&model.Model{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"display_name": displayName,
			"enabled":      enabled,
			"test_enabled": testEnabled,
		}).Error
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
