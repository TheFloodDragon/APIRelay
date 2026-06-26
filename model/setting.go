package model

import (
	"encoding/json"
	"sync"
)

// Setting 是全局键值配置表。
type Setting struct {
	Key   string `json:"key" gorm:"primaryKey;size:64"`
	Value string `json:"value" gorm:"type:text"`
}

// 全局协议规则的 Setting key。
const SettingKeyProtocolRules = "protocol_rules"

// GetSetting 读取设置值，不存在返回空字符串。
func GetSetting(key string) (string, error) {
	if DB == nil {
		return "", nil
	}
	var s Setting
	err := DB.Where("`key` = ?", key).First(&s).Error
	if err != nil {
		// 不存在视为空值
		return "", nil
	}
	return s.Value, nil
}

// SetSetting 写入（upsert）设置值。
func SetSetting(key, value string) error {
	s := Setting{Key: key, Value: value}
	err := DB.Save(&s).Error
	if err == nil && key == SettingKeyProtocolRules {
		invalidateProtocolRulesCache()
	}
	return err
}

// ---- 全局协议规则缓存 ----

var (
	protocolRulesMu     sync.RWMutex
	protocolRulesCache  []ProtocolRule
	protocolRulesLoaded bool
)

// GetGlobalProtocolRules 返回全局协议规则，带内存缓存。
func GetGlobalProtocolRules() []ProtocolRule {
	protocolRulesMu.RLock()
	if protocolRulesLoaded {
		rules := protocolRulesCache
		protocolRulesMu.RUnlock()
		return rules
	}
	protocolRulesMu.RUnlock()

	protocolRulesMu.Lock()
	defer protocolRulesMu.Unlock()
	if protocolRulesLoaded { // double-check
		return protocolRulesCache
	}
	raw, _ := GetSetting(SettingKeyProtocolRules)
	var rules []ProtocolRule
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &rules)
	}
	protocolRulesCache = rules
	protocolRulesLoaded = true
	return rules
}

func invalidateProtocolRulesCache() {
	protocolRulesMu.Lock()
	protocolRulesLoaded = false
	protocolRulesCache = nil
	protocolRulesMu.Unlock()
}
