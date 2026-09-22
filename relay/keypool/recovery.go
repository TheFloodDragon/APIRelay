package keypool

import (
	"context"
	"sync"
	"time"

	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"

	"go.uber.org/zap"
)

// ProbeFunc 是对单个失效 Key 的探测函数。返回 nil 表示该 Key 已恢复可用。
//
// 探测逻辑（构造上游请求、按 Key overlay 鉴权）位于 relay 主包，为避免
// keypool ← relay 的循环依赖，这里以依赖注入的方式接收探测实现。
type ProbeFunc func(ctx context.Context, ch *model.Channel, key *model.ChannelKey) error

// RecoveryConfig 主动探测恢复配置（由 config.KeyRecoveryConfig 转换而来）。
type RecoveryConfig struct {
	Enabled           bool
	IntervalSeconds   int
	MaxBackoffSeconds int
}

func (c RecoveryConfig) normalized() RecoveryConfig {
	if c.IntervalSeconds <= 0 {
		c.IntervalSeconds = 60
	}
	if c.MaxBackoffSeconds <= 0 {
		c.MaxBackoffSeconds = 900
	}
	return c
}

// 每轮最多探测的 Key 数，避免一次扫描打出过多上游请求。
const recoveryScanLimit = 100

var (
	recoveryCancel context.CancelFunc
	recoveryWG     sync.WaitGroup
	recoveryMu     sync.Mutex

	// backoffState 记录每个失效 Key 的下次可探测时刻与已尝试次数（指数退避）。
	backoffMu    sync.Mutex
	backoffState = map[int]*keyBackoff{}
)

type keyBackoff struct {
	nextAttemptMs int64
	attempts      int
}

// StartRecoveryWorker 启动失效 Key 的主动探测恢复 worker（幂等）。
// cfg.Enabled 为 false 或 probe 为 nil 时不启动。
func StartRecoveryWorker(cfg RecoveryConfig, probe ProbeFunc) {
	if !cfg.Enabled || probe == nil {
		return
	}
	cfg = cfg.normalized()

	recoveryMu.Lock()
	defer recoveryMu.Unlock()
	if recoveryCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	recoveryCancel = cancel

	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	recoveryWG.Add(1)
	go func() {
		defer recoveryWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runRecoveryScan(ctx, cfg, probe)
			}
		}
	}()
	logger.L().Info("key recovery worker started",
		zap.Int("interval_seconds", cfg.IntervalSeconds),
		zap.Int("max_backoff_seconds", cfg.MaxBackoffSeconds),
	)
}

// StopRecoveryWorker 停止探测 worker 并等待当前扫描结束。
func StopRecoveryWorker() {
	recoveryMu.Lock()
	cancel := recoveryCancel
	recoveryCancel = nil
	recoveryMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	recoveryWG.Wait()
	logger.L().Info("key recovery worker stopped")
}

// runRecoveryScan 扫描一批失效 Key 并按退避策略探测。
func runRecoveryScan(ctx context.Context, cfg RecoveryConfig, probe ProbeFunc) {
	keys, err := model.ListRecoverableChannelKeys(recoveryScanLimit)
	if err != nil {
		logger.L().Warn("key recovery: list recoverable keys failed", zap.Error(err))
		return
	}
	if len(keys) == 0 {
		return
	}
	mgr := GetManager()
	nowMs := time.Now().UnixMilli()

	// 渠道信息按需缓存，避免同渠道多个 Key 重复查库。
	channelCache := map[int]*model.Channel{}

	for _, key := range keys {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if key == nil || key.Id <= 0 {
			continue
		}
		if !backoffReady(key.Id, nowMs) {
			continue
		}

		ch, ok := channelCache[key.ChannelId]
		if !ok {
			ch, err = model.GetChannelByID(key.ChannelId)
			if err != nil {
				// 渠道已删除或不可读：清理退避状态，跳过。
				clearBackoff(key.Id)
				continue
			}
			channelCache[key.ChannelId] = ch
		}
		// 渠道已被管理员禁用时不探测（渠道级不可用优先）。
		if ch.Status != model.ChannelStatusEnabled {
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		perr := probe(probeCtx, ch, key)
		cancel()

		if perr == nil {
			mgr.MarkRecovered(key)
			clearBackoff(key.Id)
			logger.L().Info("key recovered via active probe",
				zap.Int("channel_id", key.ChannelId),
				zap.Int("key_id", key.Id),
				zap.Int("key_index", key.KeyIndex),
			)
			continue
		}
		// 探测失败：指数退避，等待下一轮。
		next := advanceBackoff(key.Id, nowMs, cfg)
		logger.L().Debug("key recovery probe failed",
			zap.Int("channel_id", key.ChannelId),
			zap.Int("key_id", key.Id),
			zap.Int64("next_attempt_ms", next),
			zap.Error(perr),
		)
	}
}

// backoffReady 判断某 Key 是否已过退避窗口，可再次探测。
func backoffReady(keyID int, nowMs int64) bool {
	backoffMu.Lock()
	defer backoffMu.Unlock()
	st, ok := backoffState[keyID]
	if !ok {
		return true
	}
	return nowMs >= st.nextAttemptMs
}

// advanceBackoff 在探测失败后推进退避：间隔按 2^attempts 增长，封顶 MaxBackoffSeconds。
func advanceBackoff(keyID int, nowMs int64, cfg RecoveryConfig) int64 {
	backoffMu.Lock()
	defer backoffMu.Unlock()
	st := backoffState[keyID]
	if st == nil {
		st = &keyBackoff{}
		backoffState[keyID] = st
	}
	st.attempts++
	backoffSec := cfg.IntervalSeconds << minInt(st.attempts, 16)
	if backoffSec > cfg.MaxBackoffSeconds || backoffSec <= 0 {
		backoffSec = cfg.MaxBackoffSeconds
	}
	st.nextAttemptMs = nowMs + int64(backoffSec)*1000
	return st.nextAttemptMs
}

// clearBackoff 清除某 Key 的退避状态（恢复成功或渠道消失时调用）。
func clearBackoff(keyID int) {
	backoffMu.Lock()
	delete(backoffState, keyID)
	backoffMu.Unlock()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
