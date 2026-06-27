package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 APIRelay 的全局配置。
// 加载顺序：默认值 -> config.yaml -> 环境变量覆盖。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
	Relay    RelayConfig    `yaml:"relay"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	// Driver: sqlite | mysql | postgres
	Driver string `yaml:"driver"`
	// DSN: sqlite 时为文件路径，如 ./apirelay.db
	DSN string `yaml:"dsn"`
}

type LogConfig struct {
	// Level: debug | info | warn | error
	Level string `yaml:"level"`
	// Path: 日志文件目录，为空则仅输出到 stdout
	Path string `yaml:"path"`
	// Format: json | console
	Format string `yaml:"format"`
}

type RelayConfig struct {
	// MaxRetries 单次请求跨渠道最大切换次数
	MaxRetries int `yaml:"max_retries"`
	// ChannelMaxRetries 单个渠道的重试次数
	ChannelMaxRetries int `yaml:"channel_max_retries"`
	// CooldownSeconds 渠道失败后的冷却时间（秒）
	CooldownSeconds int `yaml:"cooldown_seconds"`
	// RequestTimeout 上游请求超时（秒），0 表示不限
	RequestTimeout int `yaml:"request_timeout"`
	// DefaultGroup 令牌未指定分组时使用的默认分组
	DefaultGroup string `yaml:"default_group"`
	// CircuitBreaker 熔断器配置
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	FailureThreshold   int     `yaml:"failure_threshold"`    // 连续失败触发熔断
	SuccessThreshold   int     `yaml:"success_threshold"`    // 半开状态恢复所需成功次数
	TimeoutSeconds     int     `yaml:"timeout_seconds"`      // 熔断超时进入半开
	ErrorRateThreshold float64 `yaml:"error_rate_threshold"` // 错误率阈值
	MinRequests        int     `yaml:"min_requests"`         // 统计窗口最小请求数
}

type AuthConfig struct {
	// SessionSecret 用于管理后台会话/JWT 签名
	SessionSecret string `yaml:"session_secret"`
	// InitialRootToken 首次启动时创建的 root 管理令牌（为空则自动生成并打印）
	InitialRootToken string `yaml:"initial_root_token"`
}

// Default 返回带合理默认值的配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{Port: 3000, Host: "0.0.0.0"},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "./apirelay.db",
		},
		Log: LogConfig{Level: "info", Path: "./logs", Format: "console"},
		Relay: RelayConfig{
			MaxRetries:        3,
			ChannelMaxRetries: 1,
			CooldownSeconds:   60,
			RequestTimeout:    0,
			DefaultGroup:      "default",
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold:   5,
				SuccessThreshold:   2,
				TimeoutSeconds:     30,
				ErrorRateThreshold: 0.5,
				MinRequests:        10,
			},
		},
		Auth: AuthConfig{},
	}
}

// Load 从 path 读取 yaml（可不存在），再应用环境变量覆盖。
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

// applyEnv 使用 APIRELAY_ 前缀的环境变量覆盖配置。
func applyEnv(cfg *Config) {
	if v := os.Getenv("APIRELAY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("APIRELAY_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("APIRELAY_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("APIRELAY_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("APIRELAY_LOG_LEVEL"); v != "" {
		cfg.Log.Level = strings.ToLower(v)
	}
	if v := os.Getenv("APIRELAY_LOG_PATH"); v != "" {
		cfg.Log.Path = v
	}
	if v := os.Getenv("APIRELAY_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("APIRELAY_SESSION_SECRET"); v != "" {
		cfg.Auth.SessionSecret = v
	}
	if v := os.Getenv("APIRELAY_INITIAL_ROOT_TOKEN"); v != "" {
		cfg.Auth.InitialRootToken = v
	}
	if v := os.Getenv("APIRELAY_MAX_RETRIES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.MaxRetries = p
		}
	}
}
