package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Scheduler  SchedulerConfig  `mapstructure:"scheduler"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	CORS       CORSConfig       `mapstructure:"cors"`
}

type ServerConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Mode       string `mapstructure:"mode"`
	StaticPath string `mapstructure:"static_path"`
}

type DatabaseConfig struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

type AuthConfig struct {
	AdminKey     string `mapstructure:"admin_key"`
	RequireLogin bool   `mapstructure:"require_login"`
	JWTSecret    string `mapstructure:"jwt_secret"`
}

type SchedulerConfig struct {
	Strategy            string `mapstructure:"strategy"`
	HealthCheckInterval int    `mapstructure:"health_check_interval"`
	UnhealthyThreshold  int    `mapstructure:"unhealthy_threshold"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	RequestLog bool   `mapstructure:"request_log"`
}

type MonitoringConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
}

type PrometheusConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type CORSConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	AllowOrigins []string `mapstructure:"allow_origins"`
	AllowMethods []string `mapstructure:"allow_methods"`
	AllowHeaders []string `mapstructure:"allow_headers"`
}

var GlobalConfig *Config

//go:embed default_config.yml
var defaultConfigTemplate []byte

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 读取配置文件。默认优先使用 config.yml，并兼容旧的 config.yaml。
	configFile := configPath
	if configFile == "" {
		configFile = defaultConfigFile()
	}
	v.SetConfigFile(configFile)

	// 如果配置文件不存在，先生成默认 config.yml，方便发布产物开箱即用。
	if err := ensureConfigFile(configFile); err != nil {
		return nil, err
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	// 环境变量覆盖
	v.SetEnvPrefix("APIRELAY")
	v.AutomaticEnv()

	// 特定环境变量绑定
	v.BindEnv("server.host", "APIRELAY_HOST")
	v.BindEnv("server.port", "APIRELAY_PORT")
	v.BindEnv("auth.admin_key", "APIRELAY_ADMIN_KEY")
	v.BindEnv("database.path", "APIRELAY_DB_PATH")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 创建必要的目录
	if err := ensureDirectories(&cfg); err != nil {
		return nil, err
	}

	GlobalConfig = &cfg
	return &cfg, nil
}

func defaultConfigFile() string {
	candidates := []string{
		"config.yml",
		"config.yaml",
		"config/config.yml",
		"config/config.yaml",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "config.yml"
}

func ensureConfigFile(configFile string) error {
	if configFile == "" {
		return nil
	}

	if _, err := os.Stat(configFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查配置文件 %s 失败: %w", configFile, err)
	}

	configDir := filepath.Dir(configFile)
	if configDir != "." && configDir != "" {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("创建配置目录 %s 失败: %w", configDir, err)
		}
	}

	if err := os.WriteFile(configFile, defaultConfigTemplate, 0644); err != nil {
		return fmt.Errorf("生成默认配置文件 %s 失败: %w", configFile, err)
	}

	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 15722)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.static_path", "./web/dist")

	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.path", "./data/apirelay.db")

	v.SetDefault("auth.admin_key", "change-me-in-production")
	v.SetDefault("auth.require_login", false)
	v.SetDefault("auth.jwt_secret", "your-secret-key")

	v.SetDefault("scheduler.strategy", "priority")
	v.SetDefault("scheduler.health_check_interval", 60)
	v.SetDefault("scheduler.unhealthy_threshold", 3)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "file")
	v.SetDefault("logging.file", "./logs/apirelay.log")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 10)
	v.SetDefault("logging.max_age", 30)
	v.SetDefault("logging.request_log", true)

	v.SetDefault("monitoring.prometheus.enabled", false)
	v.SetDefault("monitoring.prometheus.path", "/metrics")

	v.SetDefault("cors.enabled", true)
	v.SetDefault("cors.allow_origins", []string{"*"})
	v.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allow_headers", []string{
		"Accept",
		"Authorization",
		"Content-Type",
		"Origin",
		"X-Requested-With",
		"X-Request-ID",
		"X-Request-Id",
		"Request-ID",
		"X-API-Key",
		"X-Api-Key",
		"X-Goog-Api-Key",
		"X-Goog-Api-Client",
		"Anthropic-Version",
		"Anthropic-Beta",
		"OpenAI-Organization",
		"OpenAI-Project",
		"OpenAI-Beta",
	})
}

func ensureDirectories(cfg *Config) error {
	dirs := []string{
		"data",
		"logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	return nil
}
