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

// UpsertChannelHealth 插入或更新渠道健康状态
func UpsertChannelHealth(h *ChannelHealth) error {
	h.UpdatedAt = time.Now()
	return DB.Save(h).Error
}

// ResetChannelHealth 重置渠道熔断器状态
func ResetChannelHealth(channelId int) error {
	return DB.Model(&ChannelHealth{}).Where("channel_id = ?", channelId).Updates(map[string]interface{}{
		"consecutive_failures":  0,
		"consecutive_successes": 0,
		"circuit_state":         CircuitClosed,
		"circuit_opened_at":     nil,
		"updated_at":            time.Now(),
	}).Error
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
