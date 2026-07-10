package model

import (
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

// DB 是全局数据库句柄。
var DB *gorm.DB

// InitDB 根据配置打开数据库连接并自动迁移表结构。
func InitDB(cfg *config.DatabaseConfig) error {
	var dialector gorm.Dialector
	switch cfg.Driver {
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	case "postgres":
		dialector = postgres.Open(cfg.DSN)
	default: // sqlite
		dialector = sqlite.Open(cfg.DSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return err
	}
	DB = db

	// SQLite 专属调优：WAL 提升读写并发、NORMAL 同步在 WAL 下安全且更快、
	// busy_timeout 缓解写锁竞争、foreign_keys 打开外键约束。
	// 纯 Go 驱动的 sqlite 写并发需限制单连接，避免 database is locked。
	// mysql/postgres 不受影响。
	if cfg.Driver != "mysql" && cfg.Driver != "postgres" {
		if err := tuneSQLite(db); err != nil {
			return err
		}
	}

	if err := migrate(); err != nil {
		return err
	}
	// 重建所有渠道的 Ability 索引，自愈历史 bool 默认值 bug 导致的脏数据
	// （旧版 Ability.Enabled 带 default:true，禁用渠道的 ability 曾被错误写为 enabled）。
	if err := ResyncAllAbilities(); err != nil {
		logger.L().Warn("resync abilities failed", zap.Error(err))
	}
	logger.L().Info("database initialized", zap.String("driver", cfg.Driver))
	return nil
}

// migrate 自动迁移所有模型。
func migrate() error {
	return DB.AutoMigrate(
		&User{},
		&Channel{},
		&Ability{},
		&Token{},
		&Log{},
		&Setting{},
		&ChannelHealth{},
	)
}

// tuneSQLite 为 sqlite 连接应用 PRAGMA 调优并限制连接池。
// 纯 Go sqlite 驱动在多写连接下易触发 "database is locked"，
// 因此写并发安全下限是单连接（SetMaxOpenConns(1)）。
func tuneSQLite(db *gorm.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if err := db.Exec(p).Error; err != nil {
			return err
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return nil
}

// now 返回当前毫秒时间戳。
func nowMilli() int64 {
	return time.Now().UnixMilli()
}
