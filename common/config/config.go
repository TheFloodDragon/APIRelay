package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath                 = "config.yaml"
	DefaultAdminMaxBodyBytes    int64 = 2 * 1024 * 1024
	DefaultRelayMaxBodyBytes    int64 = 20 * 1024 * 1024
	DefaultMaxUpstreamBodyBytes int64 = 64 * 1024 * 1024
	// 调用日志保留期默认值（仅在 log_retention.enabled 为 true 时生效）。
	DefaultLogRetentionDays            = 30
	DefaultLogPayloadRetentionDays     = 7
	DefaultLogRetentionIntervalMinutes = 60
	DefaultLogRetentionBatchSize       = 500
	maxLogRetentionBatchSize           = 10000
	DefaultInitialAdminUsername        = "admin"
	DefaultLoginMaxFailures            = 5
	DefaultLoginFailureWindowSeconds   = 10 * 60
	DefaultLoginLockoutSeconds         = 15 * 60
)

var configFilePath = struct {
	sync.RWMutex
	value string
}{value: DefaultConfigPath}

// Config 是 APIRelay 的全局配置。
// 加载顺序：默认值 -> config.yaml -> 环境变量覆盖。
type Config struct {
	Server      ServerConfig    `yaml:"server"`
	Database    DatabaseConfig  `yaml:"database"`
	LogDatabase *DatabaseConfig `yaml:"log_database,omitempty"`
	Log         LogConfig       `yaml:"log"`
	// LogRetention 调用日志表的自动清理策略。
	LogRetention LogRetentionConfig `yaml:"log_retention"`
	Relay        RelayConfig        `yaml:"relay"`
	Auth         AuthConfig         `yaml:"auth"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
	// AdminMaxBodyBytes 管理 API 请求体上限。
	AdminMaxBodyBytes int64 `yaml:"admin_max_body_bytes"`
	// CORSAllowedOrigins 管理 CORS allowlist；为空时不主动允许任何 Origin。
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	// TrustedProxies 受信任的反向代理 CIDR/IP 列表；为空时不信任任何代理，
	// ClientIP 取 RemoteAddr，避免 X-Forwarded-For 伪造绕过登录限速。
	TrustedProxies []string `yaml:"trusted_proxies"`
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

// LogRetentionConfig 控制调用日志表（logs / log_payloads）的保留期。
//
// 这些表只增不删时会无界增长（完整日志载荷是 gzip blob，单条可达数百 KB）。
// 保留期属于运维参数，放在配置文件而非 Web 设置页，避免误改成极小值造成不可逆删除。
type LogRetentionConfig struct {
	// Enabled 是否启用自动清理。默认关闭，避免升级后意外删除既有数据。
	Enabled bool `yaml:"enabled"`
	// Days 保留天数；超过该时长的日志会被删除。必须 > 0。
	Days int `yaml:"days"`
	// PayloadDays 完整日志载荷的保留天数，可比 Days 更短（载荷体积远大于摘要）。
	// <= 0 时跟随 Days。
	PayloadDays int `yaml:"payload_days"`
	// IntervalMinutes 清理任务的执行间隔（分钟）。
	IntervalMinutes int `yaml:"interval_minutes"`
	// BatchSize 单批删除的行数。SQLite 单连接下不宜过大，否则长事务会阻塞写入。
	BatchSize int `yaml:"batch_size"`
}

type RelayConfig struct {
	// MaxRetries 单次请求跨渠道最大切换次数。
	// 语义：N 次切换 = 最多尝试 N+1 个渠道（首选渠道 + N 个备用渠道）。
	MaxRetries int `yaml:"max_retries"`
	// ChannelMaxRetries 单个渠道的重试次数
	ChannelMaxRetries int `yaml:"channel_max_retries"`
	// CooldownSeconds 渠道失败后的冷却时间（秒）
	CooldownSeconds int `yaml:"cooldown_seconds"`
	// RequestTimeout 上游请求超时（秒），0 表示不限
	RequestTimeout int `yaml:"request_timeout"`
	// MaxBodyBytes Relay API 请求体上限。
	MaxBodyBytes int64 `yaml:"max_body_bytes"`
	// MaxUpstreamBodyBytes 非流式上游响应体上限。
	// 上游异常或恶意返回超大响应时防止打爆网关内存（该正文还会被解析并可能进入日志采集）。
	MaxUpstreamBodyBytes int64 `yaml:"max_upstream_body_bytes"`
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
	WindowSeconds      int     `yaml:"window_seconds"`       // 错误率统计滑动窗口秒数
}

type AuthConfig struct {
	// SessionSecret 用于管理后台会话签名；为空且允许 insecure 时会生成临时密钥。
	SessionSecret string `yaml:"session_secret"`
	// InitialRootToken 首次启动时创建的 root 管理令牌（为空则自动生成并仅打印脱敏值）
	InitialRootToken string `yaml:"initial_root_token"`
	// InitialAdminUsername 首次启动时创建的管理员用户名。
	InitialAdminUsername string `yaml:"initial_admin_username"`
	// InitialAdminPassword 首次启动时创建的管理员密码；为空时按 AllowInsecureDefaultAdmin 决定是否回退 admin123。
	InitialAdminPassword string `yaml:"initial_admin_password"`
	// AllowInsecureDefaultAdmin 是否允许回退 admin/admin123 以及临时 session secret。
	AllowInsecureDefaultAdmin bool `yaml:"allow_insecure_default_admin"`
	// LoginMaxFailures 登录失败锁定阈值。
	LoginMaxFailures int `yaml:"login_max_failures"`
	// LoginFailureWindowSeconds 登录失败计数窗口（秒）。
	LoginFailureWindowSeconds int `yaml:"login_failure_window_seconds"`
	// LoginLockoutSeconds 登录失败超过阈值后的锁定时间（秒）。
	LoginLockoutSeconds int `yaml:"login_lockout_seconds"`
}

// Default 返回带合理默认值的配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:              3000,
			Host:              "0.0.0.0",
			AdminMaxBodyBytes: DefaultAdminMaxBodyBytes,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "./apirelay.db",
		},
		Log: LogConfig{Level: "info", Path: "./logs", Format: "console"},
		LogRetention: LogRetentionConfig{
			// 默认关闭：升级到本版本不应静默删除既有日志，需运维显式开启。
			Enabled:         false,
			Days:            DefaultLogRetentionDays,
			PayloadDays:     DefaultLogPayloadRetentionDays,
			IntervalMinutes: DefaultLogRetentionIntervalMinutes,
			BatchSize:       DefaultLogRetentionBatchSize,
		},
		Relay: RelayConfig{
			MaxRetries:           3,
			ChannelMaxRetries:    1,
			CooldownSeconds:      60,
			RequestTimeout:       0,
			MaxBodyBytes:         DefaultRelayMaxBodyBytes,
			MaxUpstreamBodyBytes: DefaultMaxUpstreamBodyBytes,
			DefaultGroup:         "default",
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold:   5,
				SuccessThreshold:   2,
				TimeoutSeconds:     30,
				ErrorRateThreshold: 0.5,
				MinRequests:        10,
				WindowSeconds:      60,
			},
		},
		Auth: AuthConfig{
			InitialAdminUsername:      DefaultInitialAdminUsername,
			AllowInsecureDefaultAdmin: true,
			LoginMaxFailures:          DefaultLoginMaxFailures,
			LoginFailureWindowSeconds: DefaultLoginFailureWindowSeconds,
			LoginLockoutSeconds:       DefaultLoginLockoutSeconds,
		},
	}
}

// SetConfigFilePath 记录当前进程启动使用的配置文件路径。
func SetConfigFilePath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultConfigPath
	}
	configFilePath.Lock()
	configFilePath.value = filepath.Clean(path)
	configFilePath.Unlock()
}

// ConfigFilePath 返回当前进程启动使用的配置文件路径。
func ConfigFilePath() string {
	configFilePath.RLock()
	defer configFilePath.RUnlock()
	return configFilePath.value
}

// ValidateYAML 校验配置文件内容是否能按 APIRelay 配置结构解析。
func ValidateYAML(data []byte) error {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	cfg.Normalize()
	return nil
}

// Load 从 path 读取 yaml（可不存在），再应用环境变量覆盖。
func Load(path string) (*Config, error) {
	SetConfigFilePath(path)
	cfg := Default()

	if strings.TrimSpace(path) != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	applyEnv(cfg)
	cfg.Normalize()
	return cfg, nil
}

// Normalize 修正空值/非法值，保证运行时配置始终有安全下限。
func (c *Config) Normalize() {
	if c.Server.Port == 0 {
		c.Server.Port = 3000
	}
	if strings.TrimSpace(c.Server.Host) == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.AdminMaxBodyBytes <= 0 {
		c.Server.AdminMaxBodyBytes = DefaultAdminMaxBodyBytes
	}
	c.Server.CORSAllowedOrigins = normalizeList(c.Server.CORSAllowedOrigins)
	c.Server.TrustedProxies = normalizeList(c.Server.TrustedProxies)

	c.Database.Driver = strings.ToLower(strings.TrimSpace(c.Database.Driver))
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	c.Database.DSN = strings.TrimSpace(c.Database.DSN)
	if c.Database.DSN == "" {
		c.Database.DSN = "./apirelay.db"
	}
	if c.LogDatabase != nil {
		c.LogDatabase.Driver = strings.ToLower(strings.TrimSpace(c.LogDatabase.Driver))
		c.LogDatabase.DSN = strings.TrimSpace(c.LogDatabase.DSN)
		if c.LogDatabase.DSN == "" {
			c.LogDatabase = nil
		} else if c.LogDatabase.Driver == "" {
			c.LogDatabase.Driver = c.Database.Driver
		}
	}
	if strings.TrimSpace(c.Log.Level) == "" {
		c.Log.Level = "info"
	}
	c.Log.Level = strings.ToLower(strings.TrimSpace(c.Log.Level))
	if strings.TrimSpace(c.Log.Format) == "" {
		c.Log.Format = "console"
	}
	c.LogRetention = normalizeLogRetention(c.LogRetention)

	if c.Relay.MaxRetries <= 0 {
		c.Relay.MaxRetries = 3
	}
	if c.Relay.ChannelMaxRetries < 0 {
		c.Relay.ChannelMaxRetries = 1
	}
	if c.Relay.CooldownSeconds <= 0 {
		c.Relay.CooldownSeconds = 60
	}
	if c.Relay.MaxBodyBytes <= 0 {
		c.Relay.MaxBodyBytes = DefaultRelayMaxBodyBytes
	}
	if c.Relay.MaxUpstreamBodyBytes <= 0 {
		c.Relay.MaxUpstreamBodyBytes = DefaultMaxUpstreamBodyBytes
	}
	if strings.TrimSpace(c.Relay.DefaultGroup) == "" {
		c.Relay.DefaultGroup = "default"
	}
	if c.Relay.CircuitBreaker.FailureThreshold <= 0 {
		c.Relay.CircuitBreaker.FailureThreshold = 5
	}
	if c.Relay.CircuitBreaker.SuccessThreshold <= 0 {
		c.Relay.CircuitBreaker.SuccessThreshold = 2
	}
	if c.Relay.CircuitBreaker.TimeoutSeconds <= 0 {
		c.Relay.CircuitBreaker.TimeoutSeconds = 30
	}
	if c.Relay.CircuitBreaker.ErrorRateThreshold <= 0 {
		c.Relay.CircuitBreaker.ErrorRateThreshold = 0.5
	}
	if c.Relay.CircuitBreaker.ErrorRateThreshold > 1 {
		c.Relay.CircuitBreaker.ErrorRateThreshold = 1
	}
	if c.Relay.CircuitBreaker.MinRequests <= 0 {
		c.Relay.CircuitBreaker.MinRequests = 10
	}
	if c.Relay.CircuitBreaker.WindowSeconds <= 0 {
		c.Relay.CircuitBreaker.WindowSeconds = 60
	}

	if strings.TrimSpace(c.Auth.InitialAdminUsername) == "" {
		c.Auth.InitialAdminUsername = DefaultInitialAdminUsername
	}
	if c.Auth.LoginMaxFailures <= 0 {
		c.Auth.LoginMaxFailures = DefaultLoginMaxFailures
	}
	if c.Auth.LoginFailureWindowSeconds <= 0 {
		c.Auth.LoginFailureWindowSeconds = DefaultLoginFailureWindowSeconds
	}
	if c.Auth.LoginLockoutSeconds <= 0 {
		c.Auth.LoginLockoutSeconds = DefaultLoginLockoutSeconds
	}
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
	if v := os.Getenv("APIRELAY_ADMIN_MAX_BODY_BYTES"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Server.AdminMaxBodyBytes = p
		}
	}
	if v := os.Getenv("APIRELAY_CORS_ALLOWED_ORIGINS"); v != "" {
		cfg.Server.CORSAllowedOrigins = splitCSV(v)
	}
	if v := os.Getenv("APIRELAY_TRUSTED_PROXIES"); v != "" {
		cfg.Server.TrustedProxies = splitCSV(v)
	}
	if v := os.Getenv("APIRELAY_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("APIRELAY_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	logDBDriver, hasLogDBDriver := os.LookupEnv("APIRELAY_LOG_DB_DRIVER")
	logDBDSN, hasLogDBDSN := os.LookupEnv("APIRELAY_LOG_DB_DSN")
	if hasLogDBDriver || hasLogDBDSN {
		if cfg.LogDatabase == nil {
			cfg.LogDatabase = &DatabaseConfig{}
		}
		if hasLogDBDriver {
			cfg.LogDatabase.Driver = logDBDriver
		}
		if hasLogDBDSN {
			cfg.LogDatabase.DSN = logDBDSN
		}
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
	if v := os.Getenv("APIRELAY_LOG_RETENTION_ENABLED"); v != "" {
		if p, err := strconv.ParseBool(v); err == nil {
			cfg.LogRetention.Enabled = p
		}
	}
	if v := os.Getenv("APIRELAY_LOG_RETENTION_DAYS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogRetention.Days = p
		}
	}
	if v := os.Getenv("APIRELAY_LOG_RETENTION_PAYLOAD_DAYS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogRetention.PayloadDays = p
		}
	}
	if v := os.Getenv("APIRELAY_LOG_RETENTION_INTERVAL_MINUTES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogRetention.IntervalMinutes = p
		}
	}
	if v := os.Getenv("APIRELAY_LOG_RETENTION_BATCH_SIZE"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LogRetention.BatchSize = p
		}
	}
	if v := os.Getenv("APIRELAY_SESSION_SECRET"); v != "" {
		cfg.Auth.SessionSecret = v
	}
	if v := os.Getenv("APIRELAY_INITIAL_ROOT_TOKEN"); v != "" {
		cfg.Auth.InitialRootToken = v
	}
	if v := os.Getenv("APIRELAY_INITIAL_ADMIN_USERNAME"); v != "" {
		cfg.Auth.InitialAdminUsername = v
	}
	if v := os.Getenv("APIRELAY_INITIAL_ADMIN_PASSWORD"); v != "" {
		cfg.Auth.InitialAdminPassword = v
	}
	if v := os.Getenv("APIRELAY_ALLOW_INSECURE_DEFAULT_ADMIN"); v != "" {
		if b, ok := parseBool(v); ok {
			cfg.Auth.AllowInsecureDefaultAdmin = b
		}
	}
	if v := os.Getenv("APIRELAY_LOGIN_MAX_FAILURES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Auth.LoginMaxFailures = p
		}
	}
	if v := os.Getenv("APIRELAY_LOGIN_FAILURE_WINDOW_SECONDS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Auth.LoginFailureWindowSeconds = p
		}
	}
	if v := os.Getenv("APIRELAY_LOGIN_LOCKOUT_SECONDS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Auth.LoginLockoutSeconds = p
		}
	}
	if v := os.Getenv("APIRELAY_MAX_RETRIES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.MaxRetries = p
		}
	}
	if v := os.Getenv("APIRELAY_CHANNEL_MAX_RETRIES"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.ChannelMaxRetries = p
		}
	}
	if v := os.Getenv("APIRELAY_COOLDOWN_SECONDS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CooldownSeconds = p
		}
	}
	if v := os.Getenv("APIRELAY_REQUEST_TIMEOUT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.RequestTimeout = p
		}
	}
	if v := os.Getenv("APIRELAY_RELAY_MAX_BODY_BYTES"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Relay.MaxBodyBytes = p
		}
	}
	if v := os.Getenv("APIRELAY_MAX_UPSTREAM_BODY_BYTES"); v != "" {
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Relay.MaxUpstreamBodyBytes = p
		}
	}
	if v := os.Getenv("APIRELAY_DEFAULT_GROUP"); v != "" {
		cfg.Relay.DefaultGroup = v
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_FAILURE_THRESHOLD"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CircuitBreaker.FailureThreshold = p
		}
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_SUCCESS_THRESHOLD"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CircuitBreaker.SuccessThreshold = p
		}
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_TIMEOUT_SECONDS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CircuitBreaker.TimeoutSeconds = p
		}
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_ERROR_RATE_THRESHOLD"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Relay.CircuitBreaker.ErrorRateThreshold = p
		}
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_MIN_REQUESTS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CircuitBreaker.MinRequests = p
		}
	}
	if v := os.Getenv("APIRELAY_CIRCUIT_BREAKER_WINDOW_SECONDS"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Relay.CircuitBreaker.WindowSeconds = p
		}
	}
}

func splitCSV(s string) []string {
	return normalizeList(strings.Split(s, ","))
}

// normalizeLogRetention 补齐缺省值并限制异常输入。
//
// 关键安全约束：清理是不可逆删除，因此任何无法解释的配置都必须回退到保守默认值，
// 而不是"尽力按用户写的执行"。Days <= 0 时回退默认保留期而非删除全部数据。
func normalizeLogRetention(cfg LogRetentionConfig) LogRetentionConfig {
	if cfg.Days <= 0 {
		cfg.Days = DefaultLogRetentionDays
	}
	if cfg.PayloadDays <= 0 {
		// 未单独配置时跟随摘要保留期。
		cfg.PayloadDays = cfg.Days
	}
	if cfg.PayloadDays > cfg.Days {
		// 载荷不可能比它关联的摘要活得更久（摘要删除时会级联删除载荷）。
		cfg.PayloadDays = cfg.Days
	}
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = DefaultLogRetentionIntervalMinutes
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultLogRetentionBatchSize
	} else if cfg.BatchSize > maxLogRetentionBatchSize {
		cfg.BatchSize = maxLogRetentionBatchSize
	}
	return cfg
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}
