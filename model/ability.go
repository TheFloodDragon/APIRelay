package model

import "github.com/apirelay/apirelay/common/logger"

// Ability 是 (group, model) -> channel 的倒排索引，用于按模型快速选渠道。
type Ability struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Group     string `json:"group" gorm:"size:64;index:idx_group_model,priority:1"`
	Model     string `json:"model" gorm:"size:128;index:idx_group_model,priority:2"`
	ChannelId int    `json:"channel_id" gorm:"index"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	Priority  int    `json:"priority" gorm:"default:0;index"`
	Weight    int    `json:"weight" gorm:"default:1"`
}

// SyncChannelAbilities 重建某渠道的 Ability 索引（每个 model 一行）。
func SyncChannelAbilities(c *Channel) error {
	if err := DB.Where("channel_id = ?", c.Id).Delete(&Ability{}).Error; err != nil {
		return err
	}
	enabled := c.Status == ChannelStatusEnabled
	var abilities []Ability
	for _, m := range c.ModelList() {
		abilities = append(abilities, Ability{
			Group:     c.Group,
			Model:     m,
			ChannelId: c.Id,
			Enabled:   enabled,
			Priority:  c.Priority,
			Weight:    c.Weight,
		})
	}
	if len(abilities) == 0 {
		return nil
	}
	return DB.Create(&abilities).Error
}

// GetChannelsForModel 返回某 group+model 下、最高优先级层的可用渠道（已按 weight 排序）。
// excluded 中的渠道会被跳过（用于 failover）。
func GetChannelsForModel(group, model string, excluded map[int]struct{}) ([]*Channel, error) {
	var abilities []Ability
	err := DB.Where("`group` = ? AND model = ? AND enabled = ?", group, model, true).
		Order("priority desc, weight desc").
		Find(&abilities).Error
	if err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	// 仅取最高优先级层
	topPriority := abilities[0].Priority
	channelIDs := make([]int, 0, len(abilities))
	for _, a := range abilities {
		if a.Priority != topPriority {
			break
		}
		if excluded != nil {
			if _, skip := excluded[a.ChannelId]; skip {
				continue
			}
		}
		channelIDs = append(channelIDs, a.ChannelId)
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	var channels []*Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	logger.L().Debug("ability lookup done")
	return channels, nil
}
