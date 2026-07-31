package model

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/apirelay/apirelay/common/config"
	"gorm.io/gorm"
)

// TestSQLiteWALEnabled 验证 sqlite 分支已开启 WAL 与外键约束。
// 使用真实文件 DSN（WAL 不支持纯内存库）。
func TestSQLiteWALEnabled(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "wal_test.db")
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("init db: %v", err)
	}
	// Windows 上 TempDir 清理需先释放文件句柄；此 Cleanup 晚于 TempDir 注册，故先执行。
	t.Cleanup(func() {
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var journalMode string
	if err := DB.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	var foreignKeys int
	if err := DB.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := DB.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	// 确认 WAL 附属文件在写入后生成
	tok := &Token{Name: "wal-probe", Unlimited: true, Status: TokenStatusEnabled}
	if err := CreateToken(tok, "k-wal-probe"); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if _, err := os.Stat(dsn + "-wal"); err != nil {
		t.Errorf("expected WAL sidecar file: %v", err)
	}
}

func TestIsSQLiteBusyErr(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("database is locked"), true},
		{errors.New("database table is locked"), true},
		{errors.New("SQLITE_BUSY: database is busy"), true},
		{errors.New("some busy state"), true},
		{errors.New("quota insufficient"), false},
		{errors.New("record not found"), false},
	}
	for _, c := range cases {
		if got := isSQLiteBusyErr(c.err); got != c.want {
			t.Errorf("isSQLiteBusyErr(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestRetrySettle_RetriesOnBusy 注入一个前两次返回 busy、第三次成功的结算操作，
// 断言重试后最终成功。
func TestRetrySettle_RetriesOnBusy(t *testing.T) {
	calls := 0
	err := retrySettle(func() error {
		calls++
		if calls < 3 {
			return errors.New("database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

// TestRetrySettle_NoRetryOnNonBusy 非锁冲突错误应立即返回、不重试。
func TestRetrySettle_NoRetryOnNonBusy(t *testing.T) {
	calls := 0
	sentinel := errors.New("quota insufficient")
	err := retrySettle(func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry)", calls)
	}
}

// TestRetrySettle_ExhaustsOnPersistentBusy 持续 busy 时耗尽重试并返回最后错误。
func TestRetrySettle_ExhaustsOnPersistentBusy(t *testing.T) {
	calls := 0
	err := retrySettle(func() error {
		calls++
		return errors.New("database is locked")
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if calls != settleMaxRetries {
		t.Errorf("calls = %d, want %d", calls, settleMaxRetries)
	}
}

func TestInitDBSharesLogDatabaseByDefault(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "shared.db")
	if err := InitDB(&config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("init shared db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDatabases() })
	if DB == nil || LogDB == nil || DB != LogDB {
		t.Fatalf("shared mode handles: DB=%p LogDB=%p", DB, LogDB)
	}
	if !DB.Migrator().HasTable(&Log{}) || !DB.Migrator().HasTable(&LogPayload{}) {
		t.Fatal("shared database should contain log tables")
	}
}

func TestIndependentLogDatabaseMigratesLegacyLogsAndKeepsSource(t *testing.T) {
	dir := t.TempDir()
	primaryCfg := config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(dir, "primary.db")}
	logCfg := config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(dir, "logs.db")}

	if err := InitDB(&primaryCfg); err != nil {
		t.Fatalf("init legacy db: %v", err)
	}
	legacy := &Log{RequestId: "legacy-1", Type: LogTypeConsume, Status: 200, SrcModel: "legacy-model", CreatedAt: 1_700_000_000_000}
	if err := CreateLog(legacy); err != nil {
		t.Fatalf("create legacy log: %v", err)
	}
	if err := CreateLogPayload(legacy.Id, &FullLogData{ClientRequest: `{"hello":"世界"}`}); err != nil {
		t.Fatalf("create legacy payload: %v", err)
	}

	if err := InitDatabases(&primaryCfg, &logCfg); err != nil {
		t.Fatalf("enable independent log db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDatabases() })
	if DB == LogDB {
		t.Fatal("independent mode should use different handles")
	}

	var sourceLogs, targetLogs, targetPayloads int64
	if err := DB.Model(&Log{}).Count(&sourceLogs).Error; err != nil {
		t.Fatal(err)
	}
	if err := LogDB.Model(&Log{}).Count(&targetLogs).Error; err != nil {
		t.Fatal(err)
	}
	if err := LogDB.Model(&LogPayload{}).Count(&targetPayloads).Error; err != nil {
		t.Fatal(err)
	}
	if sourceLogs != 1 || targetLogs != 1 || targetPayloads != 1 {
		t.Fatalf("migrated counts source=%d target=%d payload=%d", sourceLogs, targetLogs, targetPayloads)
	}
	payload, err := GetLogPayload(legacy.Id)
	if err != nil || payload.ClientRequest != `{"hello":"世界"}` {
		t.Fatalf("migrated payload = %#v, err = %v", payload, err)
	}

	fresh := &Log{RequestId: "new-log", Type: LogTypeConsume, Status: 201}
	if err := CreateLog(fresh); err != nil {
		t.Fatalf("create independent log: %v", err)
	}
	if fresh.Id <= legacy.Id {
		t.Fatalf("new log id = %d, want > %d", fresh.Id, legacy.Id)
	}
	if err := DB.Model(&Log{}).Count(&sourceLogs).Error; err != nil {
		t.Fatal(err)
	}
	if err := LogDB.Model(&Log{}).Count(&targetLogs).Error; err != nil {
		t.Fatal(err)
	}
	if sourceLogs != 1 || targetLogs != 2 {
		t.Fatalf("post-cutover counts source=%d target=%d", sourceLogs, targetLogs)
	}

	for name, db := range map[string]*gorm.DB{"primary": DB, "logs": LogDB} {
		var journalMode string
		var busyTimeout int
		if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
			t.Fatalf("%s journal mode: %v", name, err)
		}
		if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
			t.Fatalf("%s busy timeout: %v", name, err)
		}
		if journalMode != "wal" || busyTimeout != 5000 {
			t.Fatalf("%s sqlite tuning mode=%q timeout=%d", name, journalMode, busyTimeout)
		}
	}

	if err := InitDatabases(&primaryCfg, &logCfg); err != nil {
		t.Fatalf("repeat independent init: %v", err)
	}
	if err := LogDB.Model(&Log{}).Count(&targetLogs).Error; err != nil {
		t.Fatal(err)
	}
	if targetLogs != 2 {
		t.Fatalf("repeat init duplicated logs: %d", targetLogs)
	}

	if err := InitDB(&primaryCfg); err == nil {
		t.Fatal("removing log database config after cutover should fail")
	}
}

func TestIndependentLogDatabaseRejectsConflictingUnclaimedTarget(t *testing.T) {
	dir := t.TempDir()
	primaryCfg := config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(dir, "primary.db")}
	logCfg := config.DatabaseConfig{Driver: "sqlite", DSN: filepath.Join(dir, "logs.db")}

	if err := InitDB(&primaryCfg); err != nil {
		t.Fatal(err)
	}
	if err := CreateLog(&Log{RequestId: "source", Type: LogTypeConsume}); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(&logCfg); err != nil {
		t.Fatal(err)
	}
	if err := CreateLog(&Log{RequestId: "target", Type: LogTypeConsume}); err != nil {
		t.Fatal(err)
	}
	if err := InitDatabases(&primaryCfg, &logCfg); err == nil {
		t.Fatal("expected conflicting source and target logs to be rejected")
	}
	_ = CloseDatabases()
}
