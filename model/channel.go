package model

import (
	"encoding/json"
	"strings"

	"github.com/apirelay/apirelay/constant"
)

// Channel 表示一个上游渠道（一个 base_url + key + 一组模型）。
type Channel struct {
	Id       int    `json:"id" gorm:"primaryKey"`
	Name     string `json:"name" gorm:"size:128"`
	Type     int    `json:"type" gorm:"default:1"` // 见 constant.ChannelType*
	Status   int    `json:"status" gorm:"default:1"`
	BaseURL  string `json:"base_url" gorm:"size:256"`
	Key      string `json:"key" gorm:"type:text"`
	Group    string `json:"group" gorm:"size:64;default:'default'"`
	Priority int    `json:"priority" gorm:"default:0;index"`
	Weight   int    `json:"weight" gorm:"default:1"`
	// Models 该渠道支持的模型，逗号分隔
	Models string `json:"models" gorm:"type:text"`
	// ModelMapping JSON：对外模型名 -> 上游真实模型名
	ModelMapping string `json:"model_mapping" gorm:"type:text"`
	// HeaderOverride JSON：附加/覆盖的上游请求头
	HeaderOverride string `json:"header_override" gorm:"type:text"`
	// CooldownUntil 冷却截止毫秒时间戳，0 表示未冷却（不持久化调度可放内存，这里持久化便于观察）
	CooldownUntil int64 `json:"cooldown_until" gorm:"default:0"`
	CreatedAt     int64 `json:"created_at"`
	UpdatedAt     int64 `json:"updated_at"`
}

const (
	ChannelStatusEnabled  = 1
	ChannelStatusDisabled = 2
)

// APIType 返回该渠道的上游协议类型。
func (c *Channel) APIType() constant.APIType {
	t, _ := constant.ChannelType2APIType(c.Type)
	return t
}

// ModelList 解析 Models 字段为切片。
func (c *Channel) ModelList() []string {
	return splitComma(c.Models)
}

// MappedModel 应用 ModelMapping，返回上游真实模型名。
func (c *Channel) MappedModel(model string) string {
	if c.ModelMapping == "" {
		return model
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(c.ModelMapping), &m); err != nil {
		return model
	}
	if v, ok := m[model]; ok && v != "" {
		return v
	}
	return model
}

// HeaderOverrideMap 解析 HeaderOverride 为 map。
func (c *Channel) HeaderOverrideMap() map[string]string {
	if c.HeaderOverride == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(c.HeaderOverride), &m); err != nil {
		return nil
	}
	return m
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// CreateChannel 创建渠道并同步 Ability 索引。
func CreateChannel(c *Channel) error {
	c.CreatedAt = nowMilli()
	c.UpdatedAt = c.CreatedAt
	if err := DB.Create(c).Error; err != nil {
		return err
	}
	return SyncChannelAbilities(c)
}

// UpdateChannel 更新渠道并同步 Ability 索引。
func UpdateChannel(c *Channel) error {
	c.UpdatedAt = nowMilli()
	if err := DB.Save(c).Error; err != nil {
		return err
	}
	return SyncChannelAbilities(c)
}

// DeleteChannel 删除渠道及其 Ability 索引。
func DeleteChannel(id int) error {
	if err := DB.Where("channel_id = ?", id).Delete(&Ability{}).Error; err != nil {
		return err
	}
	return DB.Delete(&Channel{}, id).Error
}

// GetChannelByID 按 ID 查询渠道。
func GetChannelByID(id int) (*Channel, error) {
	var c Channel
	if err := DB.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListChannels 返回全部渠道。
func ListChannels() ([]*Channel, error) {
	var list []*Channel
	err := DB.Order("priority desc, id asc").Find(&list).Error
	return list, err
}

// SetChannelCooldown 设置渠道冷却截止时间。
func SetChannelCooldown(id int, until int64) {
	DB.Model(&Channel{}).Where("id = ?", id).Update("cooldown_until", until)
}

// ClearChannelCooldown 清除渠道冷却（请求成功后调用，仅当当前确有冷却时更新）。
func ClearChannelCooldown(id int) {
	DB.Model(&Channel{}).Where("id = ? AND cooldown_until > 0", id).Update("cooldown_until", 0)
}
