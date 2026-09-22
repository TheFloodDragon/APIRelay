package keypool

import "strings"

// 分配策略。
const (
	StrategySequential = "sequential"  // 顺序优先：按 KeyIndex 升序取第一个可用 Key
	StrategyRoundRobin = "round_robin" // 轮询（预留）：per-channel 原子游标轮转
)

// Config 是 Key 轮询的运行时策略配置。
//
// 与 circuitbreaker.Config 对称，但只保留 Key 级切换所需的少量策略旋钮：
// Key 级切换很快、成本低，无需完整的错误率滑动窗口。
type Config struct {
	// Strategy 分配策略：sequential（本期）| round_robin（预留）。
	Strategy string `json:"strategy" yaml:"strategy"`
	// CooldownSeconds Key 级软冷却时长（限流无 retry-after 时、上游错误时使用）。
	CooldownSeconds int `json:"cooldown_seconds" yaml:"cooldown_seconds"`
	// FailureThreshold 连续 upstream_error/timeout 失败达此阈值则硬失效（需恢复检测）。
	FailureThreshold int `json:"failure_threshold" yaml:"failure_threshold"`
	// AuthFailDisable 鉴权失败/额度耗尽类错误是否立即硬失效（否则仅冷却）。
	AuthFailDisable bool `json:"auth_fail_disable" yaml:"auth_fail_disable"`
}

// DefaultConfig 返回默认策略配置。
func DefaultConfig() Config {
	return Config{
		Strategy:         StrategySequential,
		CooldownSeconds:  60,
		FailureThreshold: 5,
		AuthFailDisable:  true,
	}
}

// NormalizeConfig 返回清洗后的安全配置。
func NormalizeConfig(cfg Config) Config {
	return cfg.normalized()
}

func (cfg Config) normalized() Config {
	def := DefaultConfig()
	cfg.Strategy = strings.ToLower(strings.TrimSpace(cfg.Strategy))
	if cfg.Strategy != StrategyRoundRobin {
		cfg.Strategy = StrategySequential
	}
	if cfg.CooldownSeconds <= 0 {
		cfg.CooldownSeconds = def.CooldownSeconds
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = def.FailureThreshold
	}
	return cfg
}
