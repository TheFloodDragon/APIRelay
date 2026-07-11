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

// 全局日志详情配置的 Setting key。
const SettingKeyLogging = "logging"

// 全局上游网络策略的 Setting key。
const SettingKeyNetwork = "network"

// 全局模型连通性测试提示词的 Setting key。
const SettingKeyTestPrompt = "test_prompt"

// 全局模型健康统计策略的 Setting key。
const SettingKeyModelHealth = "model_health"

const DefaultTestPrompt = "Say 'hi' in one word."

const (
	DefaultModelHealthRecentCount      = 100
	DefaultModelHealthWindowHours      = 24
	DefaultModelHealthHealthyThreshold = 95.0
	DefaultModelHealthWarningThreshold = 70.0
	maxModelHealthRecentCount          = 10000
	maxModelHealthWindowHours          = 24 * 365
)

// ModelHealthConfig 是持久化的模型健康统计策略。
type ModelHealthConfig struct {
	RecentCount      int     `json:"recent_count"`
	WindowHours      int     `json:"window_hours"`
	HealthyThreshold float64 `json:"healthy_threshold"`
	WarningThreshold float64 `json:"warning_threshold"`
}

// NetworkConfig 是持久化的上游代理策略。
type NetworkConfig struct {
	Mode      string `json:"mode"`
	ManualURL string `json:"manual_url"`
	NoProxy   string `json:"no_proxy"`
}

// ModelPrice 是「模型名 -> input/output 价格（USD / 1M tokens）」的条目。
// Model 为 "default" 时作为兜底价格。
type ModelPrice struct {
	Model  string  `json:"model"`
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// LoggingConfig 是全局完整日志配置。
// 开启后记录请求/响应 headers/body 等详情，便于调试与审计。
// 敏感 headers（Authorization、Cookie 等）始终脱敏。
type LoggingConfig struct {
	Enabled               bool     `json:"enabled"`
	SanitizedHeaderKeys   []string `json:"sanitized_header_keys"`   // 需脱敏的 header 键（默认含 Authorization、Cookie 等）
	RecordClientRequest   bool     `json:"record_client_request"`   // 记录客户端请求（method/path/query/headers/body）
	RecordUpstreamRequest bool     `json:"record_upstream_request"` // 记录最终上游请求（URL/headers/body）
	RecordUpstreamResp    bool     `json:"record_upstream_resp"`    // 记录上游响应（status/headers/body）
	RecordClientResp      bool     `json:"record_client_resp"`      // 记录返回客户端响应（status/headers/body）
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
		case SettingKeyLogging:
			invalidateLoggingConfigCache()
		case SettingKeyModelHealth:
			invalidateModelHealthConfigCache()
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

// ---- 全局日志详情配置缓存 ----

var (
	loggingConfigMu     sync.RWMutex
	loggingConfigCache  *LoggingConfig
	loggingConfigLoaded bool
)

// NormalizeLoggingConfig 强制保留凭据类请求头脱敏，避免管理 API 将敏感值明文落库。
func NormalizeLoggingConfig(cfg LoggingConfig) LoggingConfig {
	mandatory := []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-API-Key"}
	seen := make(map[string]struct{}, len(cfg.SanitizedHeaderKeys)+len(mandatory))
	keys := make([]string, 0, len(cfg.SanitizedHeaderKeys)+len(mandatory))
	for _, key := range append(mandatory, cfg.SanitizedHeaderKeys...) {
		if key == "" {
			continue
		}
		normalized := key
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		keys = append(keys, key)
	}
	cfg.SanitizedHeaderKeys = keys
	return cfg
}

// GetLoggingConfig 返回全局日志配置，带内存缓存。
func GetLoggingConfig() *LoggingConfig {
	loggingConfigMu.RLock()
	if loggingConfigLoaded {
		cfg := loggingConfigCache
		loggingConfigMu.RUnlock()
		return cfg
	}
	loggingConfigMu.RUnlock()

	loggingConfigMu.Lock()
	defer loggingConfigMu.Unlock()
	if loggingConfigLoaded { // double-check
		return loggingConfigCache
	}
	raw, _ := GetSetting(SettingKeyLogging)
	var cfg LoggingConfig
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	} else {
		// 默认值：关闭完整日志，敏感 headers 需脱敏
		cfg = LoggingConfig{
			Enabled: false,
			SanitizedHeaderKeys: []string{
				"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-API-Key",
			},
			RecordClientRequest:   true,
			RecordUpstreamRequest: true,
			RecordUpstreamResp:    true,
			RecordClientResp:      true,
		}
	}
	cfg = NormalizeLoggingConfig(cfg)
	loggingConfigCache = &cfg
	loggingConfigLoaded = true
	return &cfg
}

func invalidateLoggingConfigCache() {
	loggingConfigMu.Lock()
	loggingConfigLoaded = false
	loggingConfigCache = nil
	loggingConfigMu.Unlock()
}

// ---- 全局模型健康策略缓存 ----

var (
	modelHealthConfigMu     sync.RWMutex
	modelHealthConfigCache  ModelHealthConfig
	modelHealthConfigLoaded bool
)

// NormalizeModelHealthConfig 为缺失值补默认值，并限制会导致过大查询或无效阈值的配置。
func NormalizeModelHealthConfig(cfg ModelHealthConfig) ModelHealthConfig {
	if cfg.RecentCount <= 0 {
		cfg.RecentCount = DefaultModelHealthRecentCount
	} else if cfg.RecentCount > maxModelHealthRecentCount {
		cfg.RecentCount = maxModelHealthRecentCount
	}
	if cfg.WindowHours <= 0 {
		cfg.WindowHours = DefaultModelHealthWindowHours
	} else if cfg.WindowHours > maxModelHealthWindowHours {
		cfg.WindowHours = maxModelHealthWindowHours
	}
	if cfg.HealthyThreshold <= 0 {
		cfg.HealthyThreshold = DefaultModelHealthHealthyThreshold
	} else if cfg.HealthyThreshold > 100 {
		cfg.HealthyThreshold = 100
	}
	if cfg.WarningThreshold <= 0 {
		cfg.WarningThreshold = DefaultModelHealthWarningThreshold
	} else if cfg.WarningThreshold > 100 {
		cfg.WarningThreshold = 100
	}
	if cfg.WarningThreshold > cfg.HealthyThreshold {
		cfg.WarningThreshold = cfg.HealthyThreshold
	}
	return cfg
}

// GetModelHealthConfig 返回全局模型健康统计策略，带内存缓存。
func GetModelHealthConfig() ModelHealthConfig {
	modelHealthConfigMu.RLock()
	if modelHealthConfigLoaded {
		cfg := modelHealthConfigCache
		modelHealthConfigMu.RUnlock()
		return cfg
	}
	modelHealthConfigMu.RUnlock()

	modelHealthConfigMu.Lock()
	defer modelHealthConfigMu.Unlock()
	if modelHealthConfigLoaded {
		return modelHealthConfigCache
	}
	var cfg ModelHealthConfig
	raw, _ := GetSetting(SettingKeyModelHealth)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	cfg = NormalizeModelHealthConfig(cfg)
	modelHealthConfigCache = cfg
	modelHealthConfigLoaded = true
	return cfg
}

// SaveModelHealthConfig 归一化并持久化模型健康统计策略。
func SaveModelHealthConfig(cfg ModelHealthConfig) (ModelHealthConfig, error) {
	cfg = NormalizeModelHealthConfig(cfg)
	return cfg, SaveSettingJSON(SettingKeyModelHealth, cfg)
}

func invalidateModelHealthConfigCache() {
	modelHealthConfigMu.Lock()
	modelHealthConfigLoaded = false
	modelHealthConfigCache = ModelHealthConfig{}
	modelHealthConfigMu.Unlock()
}

// GetNetworkConfig 返回持久化的上游网络配置；未配置时默认跟随系统代理。
func GetNetworkConfig() NetworkConfig {
	cfg := NetworkConfig{Mode: "system"}
	raw, _ := GetSetting(SettingKeyNetwork)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	if cfg.Mode == "" {
		cfg.Mode = "system"
	}
	return cfg
}

// SaveNetworkConfig 保存上游网络配置。
func SaveNetworkConfig(cfg NetworkConfig) error {
	return SaveSettingJSON(SettingKeyNetwork, cfg)
}

// GetTestPrompt 返回全局测试提示词；未设置时使用稳定的短回复提示。
func GetTestPrompt() string {
	value, _ := GetSetting(SettingKeyTestPrompt)
	if value == "" {
		return DefaultTestPrompt
	}
	return value
}

// SaveTestPrompt 保存全局测试提示词。
func SaveTestPrompt(value string) error {
	return SetSetting(SettingKeyTestPrompt, value)
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
