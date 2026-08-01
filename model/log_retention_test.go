package model

import (
	"context"
	"testing"
	"time"

	"github.com/apirelay/apirelay/common/config"
)

func retentionTestConfig(days, payloadDays int) config.LogRetentionConfig {
	return config.LogRetentionConfig{
		Enabled:         true,
		Days:            days,
		PayloadDays:     payloadDays,
		IntervalMinutes: 60,
		BatchSize:       2, // 故意设小，强制走多批路径
	}
}

func seedLogWithPayload(t *testing.T, l *Log, body string) *Log {
	t.Helper()
	seedLog(t, l)
	if err := CreateLogPayload(l.Id, &FullLogData{ClientRequest: body}); err != nil {
		t.Fatalf("create payload: %v", err)
	}
	return l
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := LogDB.Model(&Log{}).Count(&n).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	return n
}

func countPayloads(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := LogDB.Model(&LogPayload{}).Count(&n).Error; err != nil {
		t.Fatalf("count payloads: %v", err)
	}
	return n
}

func daysAgo(n int) int64 {
	return time.Now().AddDate(0, 0, -n).UnixMilli()
}

// 超过保留期的日志及其载荷都应被删除，期内的必须保留。
func TestPurgeExpiredLogsRemovesOnlyExpiredRows(t *testing.T) {
	setupLogTestDB(t)
	seedLogWithPayload(t, &Log{RequestId: "old-1", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(40)}, "old body 1")
	seedLogWithPayload(t, &Log{RequestId: "old-2", Type: LogTypeError, SrcModel: "m", Status: 500, CreatedAt: daysAgo(35)}, "old body 2")
	seedLogWithPayload(t, &Log{RequestId: "old-3", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(31)}, "old body 3")
	fresh := seedLogWithPayload(t, &Log{RequestId: "fresh", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(1)}, "fresh body")

	if got := countLogs(t); got != 4 {
		t.Fatalf("seeded logs = %d, want 4", got)
	}

	stats, err := PurgeExpiredLogs(context.Background(), retentionTestConfig(30, 30))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if stats.LogsDeleted != 3 {
		t.Fatalf("logs deleted = %d, want 3", stats.LogsDeleted)
	}
	if got := countLogs(t); got != 1 {
		t.Fatalf("remaining logs = %d, want 1", got)
	}
	if got := countPayloads(t); got != 1 {
		t.Fatalf("remaining payloads = %d, want 1 (orphans must not survive)", got)
	}

	// 保留下来的必须是期内那条，且详情仍可读。
	remaining, err := GetLogByID(fresh.Id)
	if err != nil {
		t.Fatalf("fresh log should survive: %v", err)
	}
	if remaining.RequestId != "fresh" {
		t.Fatalf("surviving log = %q, want fresh", remaining.RequestId)
	}
	payload, err := GetLogPayload(fresh.Id)
	if err != nil {
		t.Fatalf("fresh payload should survive: %v", err)
	}
	if payload.ClientRequest != "fresh body" {
		t.Fatalf("payload = %+v", payload)
	}
}

// PayloadDays 短于 Days 时，中间区间只删载荷、保留摘要，
// 且 has_full_record 必须复位，否则详情接口会指向已删除的载荷。
func TestPurgeExpiredLogsDropsPayloadsButKeepsSummaries(t *testing.T) {
	setupLogTestDB(t)
	mid := seedLogWithPayload(t, &Log{RequestId: "mid", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(10)}, "mid body")
	fresh := seedLogWithPayload(t, &Log{RequestId: "fresh", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(1)}, "fresh body")

	stats, err := PurgeExpiredLogs(context.Background(), retentionTestConfig(30, 7))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if stats.LogsDeleted != 0 {
		t.Fatalf("logs deleted = %d, want 0 (within retention days)", stats.LogsDeleted)
	}
	if stats.PayloadsDeleted != 1 {
		t.Fatalf("payloads deleted = %d, want 1", stats.PayloadsDeleted)
	}
	if got := countLogs(t); got != 2 {
		t.Fatalf("logs = %d, want 2", got)
	}

	midLog, err := GetLogByID(mid.Id)
	if err != nil {
		t.Fatalf("mid log should survive: %v", err)
	}
	if midLog.HasFullRecord {
		t.Fatal("has_full_record must be reset after its payload was purged")
	}
	if midLog.PayloadCompressedSize != 0 || midLog.PayloadOriginalSize != 0 {
		t.Fatalf("payload size fields not reset: %+v", midLog)
	}

	// 期内载荷不受影响。
	freshLog, err := GetLogByID(fresh.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !freshLog.HasFullRecord {
		t.Fatal("fresh log should keep its payload")
	}
}

// 反复执行必须收敛：第二次不应再删任何行（否则说明标记未复位造成死循环风险）。
func TestPurgeExpiredLogsIsIdempotent(t *testing.T) {
	setupLogTestDB(t)
	seedLogWithPayload(t, &Log{RequestId: "mid", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(10)}, "mid body")
	seedLogWithPayload(t, &Log{RequestId: "old", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(40)}, "old body")

	cfg := retentionTestConfig(30, 7)
	if _, err := PurgeExpiredLogs(context.Background(), cfg); err != nil {
		t.Fatalf("first purge: %v", err)
	}
	logsAfterFirst := countLogs(t)
	payloadsAfterFirst := countPayloads(t)

	stats, err := PurgeExpiredLogs(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second purge: %v", err)
	}
	if stats.LogsDeleted != 0 || stats.PayloadsDeleted != 0 {
		t.Fatalf("second purge deleted rows: %+v", stats)
	}
	if countLogs(t) != logsAfterFirst || countPayloads(t) != payloadsAfterFirst {
		t.Fatal("second purge changed row counts")
	}
}

// Days <= 0 会把 cutoff 推到当前时间之后，等于清空全表。
// config 归一化通常会挡住，但删除不可逆，这里必须硬拒绝。
func TestPurgeExpiredLogsRejectsNonPositiveDays(t *testing.T) {
	setupLogTestDB(t)
	seedLog(t, &Log{RequestId: "keep", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(1)})

	cfg := retentionTestConfig(30, 7)
	cfg.Days = 0
	if _, err := PurgeExpiredLogs(context.Background(), cfg); err == nil {
		t.Fatal("expected error for zero retention days")
	}
	if got := countLogs(t); got != 1 {
		t.Fatalf("logs = %d, want 1 (nothing may be deleted)", got)
	}

	cfg.Days = -5
	if _, err := PurgeExpiredLogs(context.Background(), cfg); err == nil {
		t.Fatal("expected error for negative retention days")
	}
	if got := countLogs(t); got != 1 {
		t.Fatalf("logs = %d, want 1", got)
	}
}

// 已取消的 context 应尽早返回，不继续删除。
func TestPurgeExpiredLogsHonorsCanceledContext(t *testing.T) {
	setupLogTestDB(t)
	seedLog(t, &Log{RequestId: "old", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(40)})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PurgeExpiredLogs(ctx, retentionTestConfig(30, 30)); err == nil {
		t.Fatal("expected context error")
	}
	if got := countLogs(t); got != 1 {
		t.Fatalf("logs = %d, want 1 (canceled purge must not delete)", got)
	}
}

// 分批删除必须能处理超过单批容量的数据量。
func TestPurgeExpiredLogsProcessesMultipleBatches(t *testing.T) {
	setupLogTestDB(t)
	const expired = 7 // BatchSize=2，需要多轮
	for i := 0; i < expired; i++ {
		seedLogWithPayload(t, &Log{
			RequestId: "old", Type: LogTypeConsume, SrcModel: "m", Status: 200,
			CreatedAt: daysAgo(40 + i),
		}, "body")
	}
	seedLog(t, &Log{RequestId: "fresh", Type: LogTypeConsume, SrcModel: "m", Status: 200, CreatedAt: daysAgo(1)})

	stats, err := PurgeExpiredLogs(context.Background(), retentionTestConfig(30, 30))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if stats.LogsDeleted != expired {
		t.Fatalf("logs deleted = %d, want %d", stats.LogsDeleted, expired)
	}
	if got := countLogs(t); got != 1 {
		t.Fatalf("remaining logs = %d, want 1", got)
	}
	if got := countPayloads(t); got != 0 {
		t.Fatalf("remaining payloads = %d, want 0", got)
	}
}

// 日志查询几乎总是「时间范围 + 某维度」的组合，必须有对应的复合索引，
// 否则大表上会退化成扫一大段时间区间再逐行过滤。
// 这些索引由 struct tag 声明、AutoMigrate 创建，此测试确认它们真的落到了库里。
func TestLogTableHasCompositeIndexes(t *testing.T) {
	setupLogTestDB(t)
	var names []string
	if err := LogDB.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='logs'",
	).Pluck("name", &names).Error; err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	existing := make(map[string]struct{}, len(names))
	for _, n := range names {
		existing[n] = struct{}{}
	}
	for _, want := range []string{
		"idx_log_created",
		"idx_log_type_created",
		"idx_log_user_created",
		"idx_log_token_created",
		"idx_log_channel_created",
		"idx_log_model_created",
		"idx_log_channel_model_created",
	} {
		if _, ok := existing[want]; !ok {
			t.Errorf("missing composite index %s (have: %v)", want, names)
		}
	}
}

// LogPayload 主键已自带唯一索引，不应再有重复的普通索引。
func TestLogPayloadHasNoRedundantIndex(t *testing.T) {
	setupLogTestDB(t)
	var names []string
	if err := LogDB.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='log_payloads' AND sql IS NOT NULL",
	).Pluck("name", &names).Error; err != nil {
		t.Fatalf("read indexes: %v", err)
	}
	for _, n := range names {
		if n == "idx_log_payloads_log_id" {
			t.Fatalf("redundant index on primary key still exists: %v", names)
		}
	}
}

// 未启用时 worker 不应启动，Stop 也必须可安全调用。
func TestLogRetentionWorkerDisabledIsNoop(t *testing.T) {
	setupLogTestDB(t)
	cfg := retentionTestConfig(30, 7)
	cfg.Enabled = false
	StartLogRetentionWorker(cfg)
	retentionMu.Lock()
	running := retentionCancel != nil
	retentionMu.Unlock()
	if running {
		t.Fatal("worker must not start when disabled")
	}
	StopLogRetentionWorker() // 不应 panic
}

// 启停必须干净：重复 Start 幂等，Stop 后可再次 Start。
func TestLogRetentionWorkerStartStopIsIdempotent(t *testing.T) {
	setupLogTestDB(t)
	cfg := retentionTestConfig(30, 7)
	StartLogRetentionWorker(cfg)
	StartLogRetentionWorker(cfg) // 第二次应被忽略
	retentionMu.Lock()
	running := retentionCancel != nil
	retentionMu.Unlock()
	if !running {
		t.Fatal("worker should be running")
	}

	StopLogRetentionWorker()
	retentionMu.Lock()
	stopped := retentionCancel == nil
	retentionMu.Unlock()
	if !stopped {
		t.Fatal("worker should be stopped")
	}

	StartLogRetentionWorker(cfg)
	StopLogRetentionWorker()
}
