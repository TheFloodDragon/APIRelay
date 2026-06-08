package model

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库
func InitDB(dbPath string) error {
	var err error

	// 打开数据库连接
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 自动迁移表结构
	err = DB.AutoMigrate(
		&Channel{},
		&Model{},
		&APIKey{},
		&RequestLog{},
		&SystemConfig{},
		&ModelTestLog{},
	)
	if err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	// 数据迁移：旧数据 display_name 为空时填充为 name
	if err := migrateModelDisplayName(); err != nil {
		log.Printf("模型显示名迁移警告: %v", err)
	}

	// 针对 SQLite 尝试删除旧的全局 name 唯一索引（如果存在）
	if err := removeOldNameUniqueIndex(); err != nil {
		log.Printf("删除旧索引警告: %v (可能索引不存在)", err)
	}

	log.Println("数据库初始化成功")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// migrateModelDisplayName 迁移旧数据：display_name 为空时填充为 name
func migrateModelDisplayName() error {
	return DB.Model(&Model{}).
		Where("display_name = ? OR display_name IS NULL", "").
		Update("display_name", gorm.Expr("name")).Error
}

// removeOldNameUniqueIndex 删除旧的 name 唯一索引（如果存在）
func removeOldNameUniqueIndex() error {
	// SQLite 检查并删除旧索引
	var count int64
	if err := DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_models_name'").Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return DB.Exec("DROP INDEX IF EXISTS idx_models_name").Error
	}
	// 也尝试删除 GORM 自动创建的 uniqueIndex
	if err := DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='uni_models_name'").Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return DB.Exec("DROP INDEX IF EXISTS uni_models_name").Error
	}
	return nil
}
