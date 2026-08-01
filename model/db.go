package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	// 纯 Go 的 sqlite 驱动，无需 CGO
	sqlite "github.com/libtnb/sqlite"
)

// DB 是主数据库句柄；LogDB 是调用日志数据库句柄。
// 未配置独立日志库时二者指向同一个 gorm.DB。
var (
	DB    *gorm.DB
	LogDB *gorm.DB
)

const (
	settingKeyDatabaseInstanceID = "database_instance_id"
	settingKeyLogStorageID       = "log_database_storage_id"
	logStorageStateComplete      = "complete"
	logMigrationBatchSize        = 500
	payloadMigrationBatchSize    = 100
)

// logStorageMeta 将独立日志库绑定到创建它的主数据库实例。
// 主库只保存随机 StorageID，不保存日志库 DSN 或数据库凭据。
type logStorageMeta struct {
	Id                   int    `gorm:"primaryKey"`
	StorageID            string `gorm:"size:64;uniqueIndex"`
	SourceDatabaseID     string `gorm:"size:64;index"`
	State                string `gorm:"size:16"`
	MigratedLogCount     int64
	MigratedPayloadCount int64
	MigratedMaxLogID     int
	UpdatedAt            int64
}

func (logStorageMeta) TableName() string { return "log_storage_meta" }

type logTableStats struct {
	Logs     int64
	Payloads int64
	MaxLogID int
}

// InitDB 使用主数据库并让日志默认共库，保留现有调用方式。
func InitDB(cfg *config.DatabaseConfig) error {
	return InitDatabases(cfg, nil)
}

// InitDatabases 打开主库与可选日志库，完成结构迁移和首次历史日志复制。
func InitDatabases(primaryCfg, logCfg *config.DatabaseConfig) (err error) {
	_ = CloseDatabases()

	primary, err := normalizeDatabaseConfig(primaryCfg, nil)
	if err != nil {
		return err
	}
	primaryDB, err := openDatabase(&primary)
	if err != nil {
		return fmt.Errorf("open primary database: %w", err)
	}
	DB = primaryDB
	succeeded := false
	defer func() {
		if !succeeded {
			_ = CloseDatabases()
		}
	}()

	if primary.Driver == "sqlite" {
		if err := tuneSQLite(DB); err != nil {
			return fmt.Errorf("tune primary sqlite: %w", err)
		}
	}
	if err := migrateCore(DB); err != nil {
		return fmt.Errorf("migrate primary database: %w", err)
	}

	cutoverID, err := readSettingValue(DB, settingKeyLogStorageID)
	if err != nil {
		return fmt.Errorf("read log database marker: %w", err)
	}

	logs, separate, err := normalizeLogDatabaseConfig(logCfg, &primary)
	if err != nil {
		return err
	}
	if !separate {
		if cutoverID != "" {
			return fmt.Errorf("independent log database was previously enabled; restore log_database configuration instead of falling back to the stale primary log tables")
		}
		LogDB = DB
		if err := migrateLogs(LogDB, false); err != nil {
			return fmt.Errorf("migrate shared log tables: %w", err)
		}
	} else {
		logDB, err := openDatabase(&logs)
		if err != nil {
			return fmt.Errorf("open log database: %w", err)
		}
		LogDB = logDB
		if logs.Driver == "sqlite" {
			if err := tuneSQLite(LogDB); err != nil {
				return fmt.Errorf("tune log sqlite: %w", err)
			}
		}
		if err := migrateLogs(LogDB, true); err != nil {
			return fmt.Errorf("migrate log database: %w", err)
		}
		if err := initializeIndependentLogStorage(cutoverID); err != nil {
			return err
		}
	}

	// 重建所有渠道的 Ability 索引，自愈历史 bool 默认值 bug 导致的脏数据。
	if err := ResyncAllAbilities(); err != nil {
		logger.L().Warn("resync abilities failed", zap.Error(err))
	}
	logger.L().Info("database initialized",
		zap.String("driver", primary.Driver),
		zap.Bool("independent_log_database", separate),
		zap.String("log_driver", logs.Driver),
	)
	succeeded = true
	return nil
}

func normalizeDatabaseConfig(cfg *config.DatabaseConfig, fallback *config.DatabaseConfig) (config.DatabaseConfig, error) {
	var normalized config.DatabaseConfig
	if cfg != nil {
		normalized = *cfg
	}
	normalized.Driver = strings.ToLower(strings.TrimSpace(normalized.Driver))
	normalized.DSN = strings.TrimSpace(normalized.DSN)
	if fallback != nil {
		if normalized.Driver == "" {
			normalized.Driver = fallback.Driver
		}
	} else {
		if normalized.Driver == "" {
			normalized.Driver = "sqlite"
		}
		if normalized.DSN == "" {
			normalized.DSN = "./apirelay.db"
		}
	}
	if normalized.DSN == "" {
		return config.DatabaseConfig{}, errors.New("database DSN is required")
	}
	switch normalized.Driver {
	case "sqlite", "mysql", "postgres":
		return normalized, nil
	default:
		return config.DatabaseConfig{}, fmt.Errorf("unsupported database driver %q", normalized.Driver)
	}
}

func normalizeLogDatabaseConfig(cfg, primary *config.DatabaseConfig) (config.DatabaseConfig, bool, error) {
	if cfg == nil || strings.TrimSpace(cfg.DSN) == "" {
		return *primary, false, nil
	}
	normalized, err := normalizeDatabaseConfig(cfg, primary)
	if err != nil {
		return config.DatabaseConfig{}, false, fmt.Errorf("invalid log database config: %w", err)
	}
	if sameDatabaseConfig(primary, &normalized) {
		return *primary, false, nil
	}
	return normalized, true, nil
}

func sameDatabaseConfig(left, right *config.DatabaseConfig) bool {
	if left == nil || right == nil || left.Driver != right.Driver {
		return false
	}
	return canonicalDatabaseDSN(left.Driver, left.DSN) == canonicalDatabaseDSN(right.Driver, right.DSN)
}

func canonicalDatabaseDSN(driver, dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if driver != "sqlite" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return dsn
	}
	absolute, err := filepath.Abs(filepath.Clean(dsn))
	if err != nil {
		return filepath.Clean(dsn)
	}
	return absolute
}

func openDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
	return gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
}

func migrateCore(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Channel{},
		&Ability{},
		&Token{},
		&Setting{},
		&ChannelHealth{},
	)
}

func migrateLogs(db *gorm.DB, includeStorageMeta bool) error {
	models := []any{&Log{}, &LogPayload{}}
	if includeStorageMeta {
		models = append(models, &logStorageMeta{})
	}
	return db.AutoMigrate(models...)
}

func initializeIndependentLogStorage(cutoverID string) error {
	sourceDatabaseID, err := ensureDatabaseInstanceID()
	if err != nil {
		return fmt.Errorf("ensure primary database identity: %w", err)
	}

	var meta logStorageMeta
	metaErr := LogDB.First(&meta, 1).Error
	if cutoverID != "" {
		if errors.Is(metaErr, gorm.ErrRecordNotFound) {
			return errors.New("configured log database does not contain the expected storage marker; refusing to start with an empty or replaced log database")
		}
		if metaErr != nil {
			return fmt.Errorf("read log storage marker: %w", metaErr)
		}
		if meta.State != logStorageStateComplete || meta.StorageID != cutoverID || meta.SourceDatabaseID != sourceDatabaseID {
			return errors.New("configured log database does not match the primary database cutover marker")
		}
		return nil
	}

	// 若目标端已经有完成标记而主库没有，通常是目标提交成功、主库标记写入前进程退出。
	if metaErr == nil {
		if meta.State != logStorageStateComplete || meta.StorageID == "" || meta.SourceDatabaseID != sourceDatabaseID {
			return errors.New("log database already belongs to another primary database or has an incomplete migration")
		}
		if err := writeSettingValue(DB, settingKeyLogStorageID, meta.StorageID); err != nil {
			return fmt.Errorf("recover primary log database marker: %w", err)
		}
		return nil
	}
	if !errors.Is(metaErr, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read log storage metadata: %w", metaErr)
	}

	sourceStats, err := inspectLegacyLogTables(DB)
	if err != nil {
		return fmt.Errorf("inspect primary log tables: %w", err)
	}
	targetStats, err := inspectLogTables(LogDB)
	if err != nil {
		return fmt.Errorf("inspect target log tables: %w", err)
	}
	if (targetStats.Logs > 0 || targetStats.Payloads > 0) && (sourceStats.Logs > 0 || sourceStats.Payloads > 0) {
		return errors.New("both primary and target databases contain unclaimed logs; refusing to merge potentially conflicting log IDs")
	}
	if sourceStats.Logs == 0 && sourceStats.Payloads > 0 {
		return errors.New("primary database contains log payloads without a logs table; refusing unsafe migration")
	}

	storageID, err := newStorageID()
	if err != nil {
		return fmt.Errorf("generate log storage identity: %w", err)
	}
	meta = logStorageMeta{
		Id:               1,
		StorageID:        storageID,
		SourceDatabaseID: sourceDatabaseID,
		State:            logStorageStateComplete,
		UpdatedAt:        nowMilli(),
	}

	if targetStats.Logs == 0 && targetStats.Payloads == 0 {
		if err := migrateLegacyLogs(sourceStats, &meta); err != nil {
			return fmt.Errorf("migrate legacy logs: %w", err)
		}
	} else {
		meta.MigratedLogCount = targetStats.Logs
		meta.MigratedPayloadCount = targetStats.Payloads
		meta.MigratedMaxLogID = targetStats.MaxLogID
		if err := LogDB.Transaction(func(tx *gorm.DB) error {
			if err := alignLogAutoIncrement(tx, targetStats.MaxLogID); err != nil {
				return err
			}
			return tx.Create(&meta).Error
		}); err != nil {
			return fmt.Errorf("adopt existing log database: %w", err)
		}
	}

	if err := writeSettingValue(DB, settingKeyLogStorageID, storageID); err != nil {
		return fmt.Errorf("persist primary log database marker: %w", err)
	}
	return nil
}

func ensureDatabaseInstanceID() (string, error) {
	value, err := readSettingValue(DB, settingKeyDatabaseInstanceID)
	if err != nil {
		return "", err
	}
	if value != "" {
		return value, nil
	}
	value, err = newStorageID()
	if err != nil {
		return "", err
	}
	if err := writeSettingValue(DB, settingKeyDatabaseInstanceID, value); err != nil {
		return "", err
	}
	return value, nil
}

func newStorageID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func readSettingValue(db *gorm.DB, key string) (string, error) {
	var setting Setting
	err := db.Where(map[string]any{"key": key}).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func writeSettingValue(db *gorm.DB, key, value string) error {
	return db.Save(&Setting{Key: key, Value: value}).Error
}

func inspectLegacyLogTables(db *gorm.DB) (logTableStats, error) {
	var stats logTableStats
	if db.Migrator().HasTable(&Log{}) {
		if err := db.Model(&Log{}).Count(&stats.Logs).Error; err != nil {
			return stats, err
		}
		if stats.Logs > 0 {
			if err := db.Model(&Log{}).Select("COALESCE(MAX(id), 0)").Scan(&stats.MaxLogID).Error; err != nil {
				return stats, err
			}
		}
	}
	if db.Migrator().HasTable(&LogPayload{}) {
		if err := db.Model(&LogPayload{}).Count(&stats.Payloads).Error; err != nil {
			return stats, err
		}
	}
	return stats, nil
}

func inspectLogTables(db *gorm.DB) (logTableStats, error) {
	var stats logTableStats
	if err := db.Model(&Log{}).Count(&stats.Logs).Error; err != nil {
		return stats, err
	}
	if stats.Logs > 0 {
		if err := db.Model(&Log{}).Select("COALESCE(MAX(id), 0)").Scan(&stats.MaxLogID).Error; err != nil {
			return stats, err
		}
	}
	if err := db.Model(&LogPayload{}).Count(&stats.Payloads).Error; err != nil {
		return stats, err
	}
	return stats, nil
}

func migrateLegacyLogs(sourceStats logTableStats, meta *logStorageMeta) error {
	return LogDB.Transaction(func(target *gorm.DB) error {
		lastID := 0
		for sourceStats.Logs > 0 {
			var batch []Log
			if err := DB.Where("id > ?", lastID).Order("id ASC").Limit(logMigrationBatchSize).Find(&batch).Error; err != nil {
				return err
			}
			if len(batch) == 0 {
				break
			}
			if err := target.Create(&batch).Error; err != nil {
				return err
			}
			lastID = batch[len(batch)-1].Id
		}

		lastPayloadID := 0
		for sourceStats.Payloads > 0 {
			var batch []LogPayload
			if err := DB.Where("log_id > ?", lastPayloadID).Order("log_id ASC").Limit(payloadMigrationBatchSize).Find(&batch).Error; err != nil {
				return err
			}
			if len(batch) == 0 {
				break
			}
			if err := target.Create(&batch).Error; err != nil {
				return err
			}
			lastPayloadID = batch[len(batch)-1].LogId
		}

		if err := alignLogAutoIncrement(target, sourceStats.MaxLogID); err != nil {
			return err
		}
		copiedStats, err := inspectLogTables(target)
		if err != nil {
			return err
		}
		if copiedStats != sourceStats {
			return fmt.Errorf("copied log stats %+v do not match source %+v", copiedStats, sourceStats)
		}
		meta.MigratedLogCount = copiedStats.Logs
		meta.MigratedPayloadCount = copiedStats.Payloads
		meta.MigratedMaxLogID = copiedStats.MaxLogID
		return target.Create(meta).Error
	})
}

func alignLogAutoIncrement(db *gorm.DB, maxID int) error {
	switch db.Dialector.Name() {
	case "postgres":
		var sequenceName string
		if err := db.Raw("SELECT pg_get_serial_sequence('logs', 'id')").Scan(&sequenceName).Error; err != nil {
			return err
		}
		if sequenceName == "" {
			return nil
		}
		value := maxID
		called := true
		if value < 1 {
			value = 1
			called = false
		}
		var result int64
		return db.Raw("SELECT setval(?, ?, ?)", sequenceName, value, called).Scan(&result).Error
	case "mysql":
		nextID := maxID + 1
		if nextID < 1 {
			nextID = 1
		}
		return db.Exec(fmt.Sprintf("ALTER TABLE logs AUTO_INCREMENT = %d", nextID)).Error
	default:
		// SQLite 在显式插入更大的 INTEGER PRIMARY KEY 时会自动推进 sqlite_sequence。
		return nil
	}
}

// CloseDatabases 关闭主库和独立日志库；共享句柄只关闭一次。
func CloseDatabases() error {
	var closeErrors []error
	if LogDB != nil && LogDB != DB {
		if sqlDB, err := LogDB.DB(); err != nil {
			closeErrors = append(closeErrors, err)
		} else if err := sqlDB.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if DB != nil {
		if sqlDB, err := DB.DB(); err != nil {
			closeErrors = append(closeErrors, err)
		} else if err := sqlDB.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	DB = nil
	LogDB = nil
	return errors.Join(closeErrors...)
}

// tuneSQLite 为 sqlite 连接应用 PRAGMA 调优并限制连接池。
// 纯 Go sqlite 驱动在多写连接下易触发 "database is locked"，
// 因此写并发安全下限是单连接（SetMaxOpenConns(1)）。
func tuneSQLite(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	// 必须先把连接池压到单连接，再执行 PRAGMA。
	// journal_mode 之外的 PRAGMA 都是 per-connection 设置，若池中存在多个连接，
	// 只有恰好被 Exec 用到的那一个会生效，其余连接的 busy_timeout/foreign_keys 均为默认值。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	// 不设 ConnMaxLifetime：连接被回收后新建的连接不会重放这些 PRAGMA，
	// busy_timeout 归零会让 SQLite BUSY 重试更容易失败。单连接场景下长期复用是期望行为。
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if err := db.Exec(pragma).Error; err != nil {
			return err
		}
	}
	return nil
}

// nowMilli 返回当前毫秒时间戳。
func nowMilli() int64 {
	return time.Now().UnixMilli()
}
