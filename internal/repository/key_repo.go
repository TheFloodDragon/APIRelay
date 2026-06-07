package repository

import (
	"github.com/yourusername/apirelay/internal/model"
	"gorm.io/gorm"
)

type APIKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// GetAll 获取所有API密钥
func (r *APIKeyRepository) GetAll() ([]model.APIKey, error) {
	var keys []model.APIKey
	err := r.db.Find(&keys).Error
	return keys, err
}

// GetByKey 根据密钥字符串获取
func (r *APIKeyRepository) GetByKey(key string) (*model.APIKey, error) {
	var apiKey model.APIKey
	err := r.db.Where("key = ? AND enabled = ?", key, true).First(&apiKey).Error
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// Create 创建API密钥
func (r *APIKeyRepository) Create(key *model.APIKey) error {
	return r.db.Create(key).Error
}

// Update 更新API密钥
func (r *APIKeyRepository) Update(key *model.APIKey) error {
	return r.db.Save(key).Error
}

// Delete 删除API密钥
func (r *APIKeyRepository) Delete(id uint) error {
	return r.db.Delete(&model.APIKey{}, id).Error
}

// UpdateLastUsed 更新最后使用时间
func (r *APIKeyRepository) UpdateLastUsed(id uint) error {
	return r.db.Model(&model.APIKey{}).Where("id = ?", id).Update("last_used", gorm.Expr("CURRENT_TIMESTAMP")).Error
}
