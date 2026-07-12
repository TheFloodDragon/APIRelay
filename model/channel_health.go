package model

import (
	"time"

	"gorm.io/gorm"
)

// CircuitState 熔断器状态
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // 正常
	CircuitOpen     CircuitState = "open"      // 熔断
	CircuitHalfOpen CircuitState = "half_open" // 半开试探
)

// ChannelHealth 渠道健康状态与熔断器统计
type ChannelHealth struct {
	ChannelId            int          `gorm:"primaryKey" json:"channel_id"`
	ConsecutiveFailures  int          `gorm:"default:0" json:"consecutive_failures"`
	ConsecutiveSuccesses int          `gorm:"default:0" json:"consecutive_successes"`
	TotalRequests        int          `gorm:"default:0" json:"total_requests"`
	FailedRequests       int          `gorm:"default:0" json:"failed_requests"`
	LastSuccessAt        *time.Time   `json:"last_success_at"`
	LastFailureAt        *time.Time   `json:"last_failure_at"`
	LastError            string       `json:"last_error"`
	CircuitState         CircuitState `gorm:"default:'closed'" json:"circuit_state"`
	CircuitOpenedAt      *time.Time   `json:"circuit_opened_at"` // 熔断开启时间
	PersistVersion       uint64       `gorm:"default:0" json:"-"`
	UpdatedAt            time.Time    `json:"updated_at"`
}

func (ChannelHealth) TableName() string {
	return "channel_health"
}

// GetChannelHealth 获取渠道健康状态，不存在则返回默认值
func GetChannelHealth(channelId int) (*ChannelHealth, error) {
	var h ChannelHealth
	err := DB.Where("channel_id = ?", channelId).First(&h).Error
	if err == gorm.ErrRecordNotFound {
		return &ChannelHealth{
			ChannelId:    channelId,
			CircuitState: CircuitClosed,
		}, nil
	}
	return &h, err
}

// UpsertChannelHealth 插入或更新渠道健康状态；版本较旧的异步快照会被忽略。
func UpsertChannelHealth(h *ChannelHealth) error {
	if h == nil {
		return nil
	}
	h.UpdatedAt = time.Now()
	values := map[string]interface{}{
		"consecutive_failures":  h.ConsecutiveFailures,
		"consecutive_successes": h.ConsecutiveSuccesses,
		"total_requests":        h.TotalRequests,
		"failed_requests":       h.FailedRequests,
		"last_success_at":       h.LastSuccessAt,
		"last_failure_at":       h.LastFailureAt,
		"last_error":            h.LastError,
		"circuit_state":         h.CircuitState,
		"circuit_opened_at":     h.CircuitOpenedAt,
		"persist_version":       h.PersistVersion,
		"updated_at":            h.UpdatedAt,
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ChannelHealth{}).
			Where("channel_id = ? AND persist_version <= ?", h.ChannelId, h.PersistVersion).
			Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}

		var count int64
		if err := tx.Model(&ChannelHealth{}).Where("channel_id = ?", h.ChannelId).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(h).Error; err != nil {
			var current ChannelHealth
			if lookupErr := tx.Where("channel_id = ?", h.ChannelId).First(&current).Error; lookupErr == nil && current.PersistVersion > h.PersistVersion {
				return nil
			}
			return err
		}
		return nil
	})
}

// ResetChannelHealth 原子清除渠道 cooldown 与全部持久化熔断运行状态。
func ResetChannelHealth(channelId int, version uint64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var channel Channel
		if err := tx.Select("id").First(&channel, channelId).Error; err != nil {
			return err
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelId).Update("cooldown_until", 0).Error; err != nil {
			return err
		}

		health := &ChannelHealth{
			ChannelId:      channelId,
			CircuitState:   CircuitClosed,
			PersistVersion: version,
			UpdatedAt:      time.Now(),
		}
		return tx.Save(health).Error
	})
}

// GetAllChannelHealthStats 获取所有渠道健康统计（用于 Dashboard）
func GetAllChannelHealthStats() (map[CircuitState]int, error) {
	var results []struct {
		CircuitState CircuitState
		Count        int
	}
	err := DB.Model(&ChannelHealth{}).Select("circuit_state, COUNT(*) as count").Group("circuit_state").Find(&results).Error
	if err != nil {
		return nil, err
	}
	stats := make(map[CircuitState]int)
	for _, r := range results {
		stats[r.CircuitState] = r.Count
	}
	return stats, nil
}
