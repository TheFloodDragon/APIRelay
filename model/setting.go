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

// 全局模型价格表的 Setting key。
const SettingKeyModelPrices = "model_prices"

// ModelPrice 是「模型名 -> input/output 价格（USD / 1M tokens）」的条目。
// Model 为 "default" 时作为兜底价格。
type ModelPrice struct {
	Model  string  `json:"model"`
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

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
	if err == nil {
		switch key {
		case SettingKeyProtocolRules:
			invalidateProtocolRulesCache()
		case SettingKeyModelPrices:
			invalidateModelPricesCache()
		}
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

// ---- 全局模型价格缓存 ----

var (
	modelPricesMu     sync.RWMutex
	modelPricesCache  []ModelPrice
	modelPricesLoaded bool
)

// GetGlobalModelPrices 返回全局模型价格表，带内存缓存。
func GetGlobalModelPrices() []ModelPrice {
	modelPricesMu.RLock()
	if modelPricesLoaded {
		prices := modelPricesCache
		modelPricesMu.RUnlock()
		return prices
	}
	modelPricesMu.RUnlock()

	modelPricesMu.Lock()
	defer modelPricesMu.Unlock()
	if modelPricesLoaded { // double-check
		return modelPricesCache
	}
	raw, _ := GetSetting(SettingKeyModelPrices)
	var prices []ModelPrice
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &prices)
	}
	modelPricesCache = prices
	modelPricesLoaded = true
	return prices
}

// LookupGlobalModelPrice 按模型名查全局价格，未命中回退 "default" 条目。
// 返回 (input, output, hit)。
func LookupGlobalModelPrice(modelName string) (float64, float64, bool) {
	prices := GetGlobalModelPrices()
	var def *ModelPrice
	for i := range prices {
		if prices[i].Model == modelName {
			return prices[i].Input, prices[i].Output, true
		}
		if prices[i].Model == "default" {
			def = &prices[i]
		}
	}
	if def != nil {
		return def.Input, def.Output, true
	}
	return 0, 0, false
}

func invalidateModelPricesCache() {
	modelPricesMu.Lock()
	modelPricesLoaded = false
	modelPricesCache = nil
	modelPricesMu.Unlock()
}

// UnmarshalSetting 将 JSON 字符串反序列化为对象
func UnmarshalSetting(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// SaveSettingJSON 将对象序列化为 JSON 并保存到 settings 表
func SaveSettingJSON(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return SetSetting(key, string(data))
}
