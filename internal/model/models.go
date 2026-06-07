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
	HealthStatus string         `json:"health_status" gorm:"size:20;default:unknown"` // unknown, healthy, unhealthy
	LastCheck    *time.Time     `json:"last_check"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Model 模型
type Model struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name" gorm:"uniqueIndex;not null;size:100"`
	ChannelID  uint      `json:"channel_id" gorm:"index"`
	Channel    *Channel  `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
	Alias      string    `json:"alias" gorm:"size:100"`
	RedirectTo string    `json:"redirect_to" gorm:"size:100"`
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"created_at"`
}

// APIKey API密钥
type APIKey struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Key           string         `json:"key" gorm:"uniqueIndex;not null;size:100"`
	Name          string         `json:"name" gorm:"size:100"`
	Enabled       bool           `json:"enabled" gorm:"default:true;index"`
	RateLimit     int            `json:"rate_limit" gorm:"default:60"`
	AllowedModels JSONStringList `json:"allowed_models" gorm:"type:text"`
	IPWhitelist   JSONStringList `json:"ip_whitelist" gorm:"type:text"`
	LastUsed      *time.Time     `json:"last_used"`
	CreatedAt     time.Time      `json:"created_at"`
}

// RequestLog 请求日志
type RequestLog struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	ChannelID      *uint     `json:"channel_id" gorm:"index"`
	Channel        *Channel  `json:"channel,omitempty" gorm:"foreignKey:ChannelID"`
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
