package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/apirelay/apirelay/constant"

	"gorm.io/gorm"
)

// Channel 表示一个上游渠道（一个 base_url + key + 一组模型）。
type Channel struct {
	Id       int    `json:"id" gorm:"primaryKey"`
	Name     string `json:"name" gorm:"size:128"`
	Type     int    `json:"type" gorm:"default:1"` // 供应商默认协议，见 constant.ChannelType*
	Status   int    `json:"status" gorm:"default:1"`
	BaseURL  string `json:"base_url" gorm:"size:256"`
	Key      string `json:"key" gorm:"type:text"`
	Group    string `json:"group" gorm:"size:64;default:'default'"`
	Priority int    `json:"priority" gorm:"default:0;index"`
	Weight   int    `json:"weight" gorm:"default:1"`
	// Models 该渠道启用模型的显示名，逗号分隔。
	// 仍保留以兼容旧数据与 /v1/models 聚合查询；保存时由 ModelConfigs 回填。
	Models string `json:"models" gorm:"type:text"`
	// ModelConfigs JSON：[]ChannelModel，每模型的启用/协议覆盖/上游名映射。
	// 为空时由 Models 派生（全部启用、无覆盖），保证向后兼容。
	ModelConfigs string `json:"model_configs" gorm:"type:text"`
	// ProtocolRules JSON：[]ProtocolRule，供应商级正则 -> 协议规则。
	ProtocolRules string `json:"protocol_rules" gorm:"type:text"`
	// ModelMapping JSON：对外模型名 -> 上游真实模型名（旧字段，作为 ModelConfigs.Upstream 的回退）。
	ModelMapping string `json:"model_mapping" gorm:"type:text"`
	// HeaderOverride JSON：附加/覆盖的上游请求头
	HeaderOverride string `json:"header_override" gorm:"type:text"`
	// CooldownUntil 冷却截止毫秒时间戳，0 表示未冷却（不持久化调度可放内存，这里持久化便于观察）
	CooldownUntil int64 `json:"cooldown_until" gorm:"default:0"`
	CreatedAt     int64 `json:"created_at"`
	UpdatedAt     int64 `json:"updated_at"`
}

// ChannelModel 描述渠道下单个模型的配置。
type ChannelModel struct {
	// Name 显示名（对外请求名，Ability 索引键）。
	Name string `json:"name"`
	// Enabled 是否启用（禁用模型不参与路由）。
	Enabled bool `json:"enabled"`
	// Protocol 协议覆盖："" 表示继承（走规则/供应商默认）；否则为 openai|anthropic|responses。
	Protocol string `json:"protocol"`
	// Upstream 上游真实模型名，"" 表示与 Name 相同。
	Upstream string `json:"upstream"`
	// Input/Output 价格（USD / 1M tokens）。0 表示继承（走全局价格表/默认）。
	Input  float64 `json:"input,omitempty"`
	Output float64 `json:"output,omitempty"`
}

// ProtocolRule 是「正则匹配显示名 -> 协议」的规则。
type ProtocolRule struct {
	Pattern  string `json:"pattern"`  // 正则表达式
	Protocol string `json:"protocol"` // openai|anthropic|responses
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

// ModelConfigList 返回模型配置列表。
// 若 ModelConfigs 为空，则从旧 Models 字段派生（全部启用、无协议覆盖），保证向后兼容。
func (c *Channel) ModelConfigList() []ChannelModel {
	if strings.TrimSpace(c.ModelConfigs) != "" {
		var list []ChannelModel
		if err := json.Unmarshal([]byte(c.ModelConfigs), &list); err == nil {
			return list
		}
	}
	// 派生：旧渠道升级路径
	names := c.ModelList()
	list := make([]ChannelModel, 0, len(names))
	for _, n := range names {
		list = append(list, ChannelModel{Name: n, Enabled: true})
	}
	return list
}

// EnabledModelConfigs 返回启用的模型配置。
func (c *Channel) EnabledModelConfigs() []ChannelModel {
	all := c.ModelConfigList()
	out := make([]ChannelModel, 0, len(all))
	for _, m := range all {
		if m.Enabled && strings.TrimSpace(m.Name) != "" {
			out = append(out, m)
		}
	}
	return out
}

// EnabledModelNames 返回启用模型的显示名列表。
func (c *Channel) EnabledModelNames() []string {
	cfgs := c.EnabledModelConfigs()
	out := make([]string, 0, len(cfgs))
	for _, m := range cfgs {
		out = append(out, m.Name)
	}
	return out
}

// ModelConfig 按显示名查找模型配置。
func (c *Channel) ModelConfig(display string) (ChannelModel, bool) {
	for _, m := range c.ModelConfigList() {
		if m.Name == display {
			return m, true
		}
	}
	return ChannelModel{}, false
}

// ProtocolRuleList 解析供应商级协议规则。
func (c *Channel) ProtocolRuleList() []ProtocolRule {
	if strings.TrimSpace(c.ProtocolRules) == "" {
		return nil
	}
	var list []ProtocolRule
	if err := json.Unmarshal([]byte(c.ProtocolRules), &list); err != nil {
		return nil
	}
	return list
}

// MappedModel 返回上游真实模型名。
// 优先使用 ModelConfigs 中的 Upstream，其次回退旧 ModelMapping，最后用原名。
func (c *Channel) MappedModel(display string) string {
	if m, ok := c.ModelConfig(display); ok && strings.TrimSpace(m.Upstream) != "" {
		return m.Upstream
	}
	if c.ModelMapping != "" {
		var mm map[string]string
		if err := json.Unmarshal([]byte(c.ModelMapping), &mm); err == nil {
			if v, ok := mm[display]; ok && v != "" {
				return v
			}
		}
	}
	return display
}

// HeaderOverrideResult 是 HeaderOverride 的解析、过滤结果。
type HeaderOverrideResult struct {
	Headers map[string]string
	Ignored []string
}

// ParseHeaderOverride 解析并校验 HeaderOverride。
// 空值合法；危险请求头会被过滤并通过 Ignored 返回。
func ParseHeaderOverride(value string) (HeaderOverrideResult, error) {
	if strings.TrimSpace(value) == "" {
		return HeaderOverrideResult{}, nil
	}

	var raw any
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return HeaderOverrideResult{}, fmt.Errorf("必须是合法的 JSON 对象: %w", err)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return HeaderOverrideResult{}, fmt.Errorf("必须是 JSON 对象")
	}

	result := HeaderOverrideResult{Headers: make(map[string]string, len(obj))}
	for k, rawValue := range obj {
		v, ok := rawValue.(string)
		if !ok {
			return HeaderOverrideResult{}, fmt.Errorf("请求头 %q 的值必须是字符串", k)
		}
		name := strings.TrimSpace(k)
		if name == "" {
			result.Ignored = append(result.Ignored, k)
			continue
		}
		canonicalName := http.CanonicalHeaderKey(name)
		if _, denied := headerOverrideDenylist[strings.ToLower(name)]; denied {
			result.Ignored = append(result.Ignored, canonicalName)
			continue
		}
		result.Headers[canonicalName] = v
	}
	if len(result.Headers) == 0 {
		result.Headers = nil
	}
	sort.Strings(result.Ignored)
	return result, nil
}

// ParseHeaderOverride 解析当前渠道的 HeaderOverride。
func (c *Channel) ParseHeaderOverride() (HeaderOverrideResult, error) {
	if c == nil {
		return HeaderOverrideResult{}, nil
	}
	return ParseHeaderOverride(c.HeaderOverride)
}

// HeaderOverrideMap 解析 HeaderOverride 为 map。
// 为保持兼容，非法配置仍返回 nil，且这里返回未过滤的原始键值。
func (c *Channel) HeaderOverrideMap() map[string]string {
	if c == nil || strings.TrimSpace(c.HeaderOverride) == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(c.HeaderOverride), &m); err != nil {
		return nil
	}
	return m
}

var headerOverrideDenylist = map[string]struct{}{
	"authorization":     {},
	"x-api-key":         {},
	"anthropic-version": {},
	"content-length":    {},
	"host":              {},
	"connection":        {},
	"transfer-encoding": {},
	"content-type":      {},
}

// SafeHeaderOverrideMap 返回过滤后的 HeaderOverride，避免覆盖鉴权、协议与传输关键请求头。
// 为保持兼容，非法配置仍返回 nil。
func (c *Channel) SafeHeaderOverrideMap() map[string]string {
	result, err := c.ParseHeaderOverride()
	if err != nil {
		return nil
	}
	return result.Headers
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
	return DB.Transaction(func(tx *gorm.DB) error {
		c.backfillModels()
		c.CreatedAt = nowMilli()
		c.UpdatedAt = c.CreatedAt
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		return syncChannelAbilitiesTx(tx, c)
	})
}

// UpdateChannel 更新渠道并同步 Ability 索引。
func UpdateChannel(c *Channel) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		c.backfillModels()
		c.UpdatedAt = nowMilli()
		if err := tx.Save(c).Error; err != nil {
			return err
		}
		return syncChannelAbilitiesTx(tx, c)
	})
}

// backfillModels 用启用模型的显示名回填 Models 字段，使旧聚合查询继续有效。
func (c *Channel) backfillModels() {
	if strings.TrimSpace(c.ModelConfigs) == "" {
		return // 未使用对象列表，保留原 Models
	}
	c.Models = strings.Join(c.EnabledModelNames(), ",")
}

// DeleteChannel 删除渠道及其 Ability 索引。
func DeleteChannel(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&Ability{}).Error; err != nil {
			return err
		}
		return tx.Delete(&Channel{}, id).Error
	})
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

// ReorderChannels 按给定 ID 顺序重排优先级（首位最高）。
// 优先级按降序分配（n-1, n-2, ... 0），并同步更新对应 Ability 索引的优先级。
func ReorderChannels(orderedIDs []int) error {
	n := len(orderedIDs)
	if n == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range orderedIDs {
			priority := n - 1 - i // 首位优先级最高
			if err := tx.Model(&Channel{}).Where("id = ?", id).
				Updates(map[string]any{"priority": priority, "updated_at": nowMilli()}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Ability{}).Where("channel_id = ?", id).
				Update("priority", priority).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetChannelCooldown 设置渠道冷却截止时间。
func SetChannelCooldown(id int, until int64) {
	DB.Model(&Channel{}).Where("id = ?", id).Update("cooldown_until", until)
}

// ClearChannelCooldown 清除渠道冷却（请求成功后调用，仅当当前确有冷却时更新）。
func ClearChannelCooldown(id int) {
	DB.Model(&Channel{}).Where("id = ? AND cooldown_until > 0", id).Update("cooldown_until", 0)
}
