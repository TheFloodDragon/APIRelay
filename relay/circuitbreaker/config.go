package circuitbreaker

// Config 熔断器全局配置
type Config struct {
	FailureThreshold   int     `json:"failure_threshold" yaml:"failure_threshold"`       // 连续失败次数阈值
	SuccessThreshold   int     `json:"success_threshold" yaml:"success_threshold"`       // 半开状态恢复所需成功次数
	TimeoutSeconds     int     `json:"timeout_seconds" yaml:"timeout_seconds"`           // 熔断后多久进入半开状态
	ErrorRateThreshold float64 `json:"error_rate_threshold" yaml:"error_rate_threshold"` // 错误率阈值 (0-1)
	MinRequests        int     `json:"min_requests" yaml:"min_requests"`                 // 统计窗口最小请求数
	WindowSeconds      int     `json:"window_seconds" yaml:"window_seconds"`             // 错误率统计滑动窗口秒数
	ChannelMaxRetries  int     `json:"channel_max_retries" yaml:"channel_max_retries"`   // 单渠道重试次数
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		FailureThreshold:   5,
		SuccessThreshold:   2,
		TimeoutSeconds:     30,
		ErrorRateThreshold: 0.5,
		MinRequests:        10,
		WindowSeconds:      60,
		ChannelMaxRetries:  1,
	}
}

// NormalizeConfig 返回清洗后的安全配置。
func NormalizeConfig(cfg Config) Config {
	return cfg.normalized()
}

func (cfg Config) normalized() Config {
	def := DefaultConfig()
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = def.FailureThreshold
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = def.SuccessThreshold
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = def.TimeoutSeconds
	}
	if cfg.ErrorRateThreshold <= 0 {
		cfg.ErrorRateThreshold = def.ErrorRateThreshold
	}
	if cfg.ErrorRateThreshold > 1 {
		cfg.ErrorRateThreshold = 1
	}
	if cfg.MinRequests <= 0 {
		cfg.MinRequests = def.MinRequests
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = def.WindowSeconds
	}
	if cfg.ChannelMaxRetries < 0 {
		cfg.ChannelMaxRetries = def.ChannelMaxRetries
	}
	return cfg
}
