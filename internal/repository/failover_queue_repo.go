package repository

import (
	"fmt"

	"github.com/TheFloodDragon/APIRelay/internal/model"
	"gorm.io/gorm"
)

type FailoverQueueRepository struct {
	db *gorm.DB
}

func GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return NewFailoverQueueRepository(model.DB).GetFailoverQueue()
}

func SaveFailoverQueue(channelIDs []uint) error {
	if model.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return NewFailoverQueueRepository(model.DB).SaveFailoverQueue(channelIDs)
}

func NewFailoverQueueRepository(db *gorm.DB) *FailoverQueueRepository {
	return &FailoverQueueRepository{db: db}
}

func (r *FailoverQueueRepository) GetFailoverQueue() ([]model.FailoverQueueItem, error) {
	var items []model.FailoverQueueItem
	err := r.db.Preload("Channel").Order("position ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *FailoverQueueRepository) SaveFailoverQueue(channelIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.FailoverQueueItem{}).Error; err != nil {
			return err
		}
		items := make([]model.FailoverQueueItem, 0, len(channelIDs))
		seen := make(map[uint]struct{}, len(channelIDs))
		for index, channelID := range channelIDs {
			if channelID == 0 {
				continue
			}
			if _, exists := seen[channelID]; exists {
				continue
			}
			seen[channelID] = struct{}{}
			items = append(items, model.FailoverQueueItem{
				ChannelID: channelID,
				Position:  index + 1,
			})
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}
