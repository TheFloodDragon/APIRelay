package model

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/apirelay/apirelay/common/config"
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
