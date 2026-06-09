package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Channel 渠道模型
type Channel struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"not null;size:100"`
	Type         string         `json:"type" gorm:"not null;size:50"` // openai, anthropic, gemini, etc.
	APIKey       string         `json:"api_key" gorm:"not null"`
	BaseURL      string         `json:"base_url" gorm:"size:255"`
	Models       JSONStringList `json:"models" gorm:"type:text"`
	Priority     int            `json:"priority" gorm:"default:0;index"`
	Weight       int            `json:"weight" gorm:"default:1"`
	Enabled      bool           `json:"enabled" gorm:"default:true;index"`
	Timeout      int            `json:"timeout" gorm:"default:60000"` // 毫秒
	MaxRetries   int            `json:"max_retries" gorm:"default:3"`
	Config       JSONMap        `json:"config" gorm:"type:text"`
	HealthStatus string         `json:"health_status" gorm:"size:20;default:unknown"` // unknown, healthy, degraded, unhealthy
	LastCheck    *time.Time     `json:"last_check"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Model 模型
type Model struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null;size:100;index:idx_channel_model,unique"` // 上游真实模型名
	DisplayName string    `json:"display_name" gorm:"size:100"`                                 // 对外调用名/显示名
	ChannelID   uint      `json:"channel_id" gorm:"index;index:idx_channel_model,unique"`
	Channel     *Channel  `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	Alias       string    `json:"alias" gorm:"size:100"`
	RedirectTo  string    `json:"redirect_to" gorm:"size:100"`
	Enabled     bool      `json:"enabled" gorm:"default:true;index"`
	CreatedAt   time.Time `json:"created_at"`
}

// APIKey API密钥
type APIKey struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Key           string         `json:"key" gorm:"uniqueIndex;not null;size:100"`
	Name          string         `json:"name" gorm:"size:100"`
	Enabled       bool           `json:"enabled" gorm:"default:true;index"`
	AllowedModels JSONStringList `json:"allowed_models" gorm:"type:text"`
	IPWhitelist   JSONStringList `json:"ip_whitelist" gorm:"type:text"`
	LastUsed      *time.Time     `json:"last_used"`
	CreatedAt     time.Time      `json:"created_at"`
}

// RequestLog 请求日志
type RequestLog struct {
	ID          uint     `json:"id" gorm:"primaryKey"`
	RequestID   string   `json:"request_id" gorm:"size:64;index"`
	ChannelID   *uint    `json:"channel_id" gorm:"index"`
	Channel     *Channel `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	ChannelType string   `json:"channel_type" gorm:"size:50;index"`
	APIType     string   `json:"api_type" gorm:"size:50;index"`
	RelayMode   string   `json:"relay_mode" gorm:"size:50;index"`
	RelayFormat string   `json:"relay_format" gorm:"size:50"`

	ResolvedModel  string    `json:"resolved_model" gorm:"size:100;index"`
	Model          string    `json:"model" gorm:"size:100;index"`
	Method         string    `json:"method" gorm:"size:20"`
	Path           string    `json:"path" gorm:"size:255"`
	StatusCode     int       `json:"status_code"`
	RequestTokens  int       `json:"request_tokens"`
	ResponseTokens int       `json:"response_tokens"`
	Latency        int       `json:"latency"` // 毫秒
	Error          string    `json:"error" gorm:"type:text"`
	IP             string    `json:"ip" gorm:"size:50"`
	APIKeyID       *uint     `json:"api_key_id" gorm:"index"`
	APIKey         *APIKey   `json:"api_key,omitempty" gorm:"foreignKey:APIKeyID"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
}

// SystemConfig 系统配置
type SystemConfig struct {
	Key       string    `json:"key" gorm:"primaryKey;size:100"`
	Value     string    `json:"value" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProxyConfig 全局代理配置。所有协议入口共享同一份配置。
type ProxyConfig struct {
	ID                        uint      `json:"id" gorm:"primaryKey"`
	Enabled                   bool      `json:"enabled"`
	AutoFailoverEnabled       bool      `json:"auto_failover_enabled"`
	MaxRetries                int       `json:"max_retries"`
	NonStreamingTimeoutMS     int       `json:"non_streaming_timeout_ms"`
	StreamingFirstByteTimeout int       `json:"streaming_first_byte_timeout"`
	StreamingIdleTimeoutMS    int       `json:"streaming_idle_timeout_ms"`
	CircuitFailureThreshold   int       `json:"circuit_failure_threshold"`
	CircuitSuccessThreshold   int       `json:"circuit_success_threshold"`
	CircuitOpenSeconds        int       `json:"circuit_open_seconds"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

// FailoverQueueItem 全局故障转移队列项。
type FailoverQueueItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ChannelID uint      `json:"channel_id" gorm:"uniqueIndex"`
	Channel   *Channel  `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	Position  int       `json:"position" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
}

// ProviderHealth 全局渠道健康状态。
type ProviderHealth struct {
	ChannelID           uint       `json:"channel_id" gorm:"primaryKey"`
	Channel             *Channel   `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	IsHealthy           bool       `json:"is_healthy"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastFailureAt       *time.Time `json:"last_failure_at"`
	LastError           string     `json:"last_error" gorm:"type:text"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// JSONStringList 用于存储字符串数组的JSON类型
type JSONStringList []string

func (j *JSONStringList) Scan(value interface{}) error {
	if value == nil {
		*j = []string{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONStringList) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "[]", nil
	}
	return json.Marshal(j)
}

// JSONMap 用于存储map的JSON类型
type JSONMap map[string]interface{}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONMap) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return json.Marshal(j)
}
