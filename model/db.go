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
	)
}

// now 返回当前毫秒时间戳。
func nowMilli() int64 {
	return time.Now().UnixMilli()
}
