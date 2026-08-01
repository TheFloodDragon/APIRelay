package model

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 调用日志保留期清理。
//
// logs / log_payloads 此前只增不删：LogPayload.CompressedData 是 gzip blob，
// Log.Error / Log.Content 是无长度上限的 text，长期运行后数据库会无界增长。
//
// 实现要点：
//   - 分批删除 + 每批独立事务：SQLite 只有一个写连接，单个大事务会阻塞所有转发请求的日志落库。
//   - 先删载荷再删摘要：反过来会留下无法被引用、也无法按时间定位的孤儿载荷
//     （log_payloads 没有 created_at，只能通过 logs 关联判断年龄）。
//   - 每批之间让出时间片，避免持续占满写连接。

// RetentionStats 一次清理的结果。
type RetentionStats struct {
	// PayloadsDeleted 删除的完整载荷行数（含仅超过 PayloadDays 的部分）。
	PayloadsDeleted int64
	// LogsDeleted 删除的日志摘要行数。
	LogsDeleted int64
}

var (
	retentionCancel context.CancelFunc
	retentionWG     sync.WaitGroup
	retentionMu     sync.Mutex
)

// StartLogRetentionWorker 启动后台清理任务（幂等）。cfg.Enabled 为 false 时不启动。
func StartLogRetentionWorker(cfg config.LogRetentionConfig) {
	if !cfg.Enabled {
		return
	}
	retentionMu.Lock()
	defer retentionMu.Unlock()
	if retentionCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	retentionCancel = cancel

	interval := time.Duration(cfg.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = time.Duration(config.DefaultLogRetentionIntervalMinutes) * time.Minute
	}

	retentionWG.Add(1)
	go func() {
		defer retentionWG.Done()
		// 启动后先等一个间隔再首次清理：避免与启动期的迁移/预热争抢写连接。
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := PurgeExpiredLogs(ctx, cfg)
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						logger.L().Warn("log retention purge failed", zap.Error(err))
					}
					continue
				}
				if stats.LogsDeleted > 0 || stats.PayloadsDeleted > 0 {
					logger.L().Info("log retention purge completed",
						zap.Int64("logs_deleted", stats.LogsDeleted),
						zap.Int64("payloads_deleted", stats.PayloadsDeleted),
						zap.Int("retention_days", cfg.Days),
						zap.Int("payload_retention_days", cfg.PayloadDays),
					)
				}
			}
		}
	}()
	logger.L().Info("log retention worker started",
		zap.Int("retention_days", cfg.Days),
		zap.Int("payload_retention_days", cfg.PayloadDays),
		zap.Int("interval_minutes", cfg.IntervalMinutes),
	)
}

// StopLogRetentionWorker 停止清理任务并等待当前批次结束。
func StopLogRetentionWorker() {
	retentionMu.Lock()
	cancel := retentionCancel
	retentionCancel = nil
	retentionMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	retentionWG.Wait()
	logger.L().Info("log retention worker stopped")
}

// PurgeExpiredLogs 执行一次清理：先删超期载荷，再删超期日志摘要。
func PurgeExpiredLogs(ctx context.Context, cfg config.LogRetentionConfig) (RetentionStats, error) {
	var stats RetentionStats
	if LogDB == nil {
		return stats, errors.New("log database is not initialized")
	}
	// 防御：Days <= 0 会把 cutoff 推到当前时间之后，等于清空全表。
	// 正常路径由 config 归一化保证，这里再挡一次，因为删除不可逆。
	if cfg.Days <= 0 {
		return stats, fmt.Errorf("invalid retention days %d", cfg.Days)
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = config.DefaultLogRetentionBatchSize
	}

	now := time.Now()

	// 1) 仅超过载荷保留期、但摘要还需保留的日志：只删载荷，保留摘要供查询。
	payloadDays := cfg.PayloadDays
	if payloadDays <= 0 || payloadDays > cfg.Days {
		payloadDays = cfg.Days
	}
	payloadCutoff := now.AddDate(0, 0, -payloadDays).UnixMilli()
	deleted, err := purgePayloadsBefore(ctx, payloadCutoff, batchSize)
	stats.PayloadsDeleted += deleted
	if err != nil {
		return stats, err
	}

	// 2) 超过摘要保留期的日志：连带其载荷一起删除。
	logCutoff := now.AddDate(0, 0, -cfg.Days).UnixMilli()
	logsDeleted, payloadsDeleted, err := purgeLogsBefore(ctx, logCutoff, batchSize)
	stats.LogsDeleted += logsDeleted
	stats.PayloadsDeleted += payloadsDeleted
	return stats, err
}

// purgePayloadsBefore 删除 created_at < cutoff 的日志所关联的载荷，但保留日志摘要。
// 同时复位摘要上的 has_full_record 与体积字段，避免详情接口指向已删除的载荷。
func purgePayloadsBefore(ctx context.Context, cutoff int64, batchSize int) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		// 只挑仍标记有载荷的行；每轮处理完会复位标记，因此循环必然收敛。
		var ids []int
		if err := LogDB.WithContext(ctx).Model(&Log{}).
			Where("created_at < ? AND has_full_record = ?", cutoff, true).
			Order("id ASC").
			Limit(batchSize).
			Pluck("id", &ids).Error; err != nil {
			return total, err
		}
		if len(ids) == 0 {
			return total, nil
		}

		var deleted int64
		err := LogDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Where("log_id IN ?", ids).Delete(&LogPayload{})
			if result.Error != nil {
				return result.Error
			}
			deleted = result.RowsAffected
			// 复位标记必须和删除处于同一事务：否则中途失败会留下
			// has_full_record=true 但载荷已消失的行，详情接口将报错。
			return tx.Model(&Log{}).Where("id IN ?", ids).Updates(map[string]any{
				"has_full_record":         false,
				"payload_original_size":   0,
				"payload_compressed_size": 0,
			}).Error
		})
		if err != nil {
			return total, err
		}
		total += deleted
		yieldBetweenBatches(ctx)
	}
}

// purgeLogsBefore 删除 created_at < cutoff 的日志及其载荷。
// 返回 (删除的摘要数, 删除的载荷数)。
func purgeLogsBefore(ctx context.Context, cutoff int64, batchSize int) (int64, int64, error) {
	var logsTotal, payloadsTotal int64
	for {
		if err := ctx.Err(); err != nil {
			return logsTotal, payloadsTotal, err
		}
		var ids []int
		if err := LogDB.WithContext(ctx).Model(&Log{}).
			Where("created_at < ?", cutoff).
			Order("id ASC").
			Limit(batchSize).
			Pluck("id", &ids).Error; err != nil {
			return logsTotal, payloadsTotal, err
		}
		if len(ids) == 0 {
			return logsTotal, payloadsTotal, nil
		}

		var logsDeleted, payloadsDeleted int64
		err := LogDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// 先删载荷：反序会留下无法按时间定位的孤儿载荷
			// （log_payloads 没有 created_at，只能靠 logs 关联判断年龄）。
			payloadResult := tx.Where("log_id IN ?", ids).Delete(&LogPayload{})
			if payloadResult.Error != nil {
				return payloadResult.Error
			}
			payloadsDeleted = payloadResult.RowsAffected

			logResult := tx.Where("id IN ?", ids).Delete(&Log{})
			if logResult.Error != nil {
				return logResult.Error
			}
			logsDeleted = logResult.RowsAffected
			return nil
		})
		if err != nil {
			return logsTotal, payloadsTotal, err
		}
		logsTotal += logsDeleted
		payloadsTotal += payloadsDeleted
		// 本批已被删除，下一轮 Pluck 自然拿到新的一批，无需游标。
		yieldBetweenBatches(ctx)
	}
}

// yieldBetweenBatches 在批次之间短暂让出，避免持续占满 SQLite 的唯一写连接。
func yieldBetweenBatches(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(retentionBatchPause):
	}
}

const retentionBatchPause = 20 * time.Millisecond
