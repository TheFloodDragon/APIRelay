package model

import (
	"strings"

	"gorm.io/gorm"
)

// quoteIdent 按当前数据库方言引用标识符。
//
// group / key 在 MySQL 与 PostgreSQL 中都是保留字，必须引用后才能出现在 WHERE 子句里。
// 但引用语法是方言相关的：MySQL/SQLite 用反引号，PostgreSQL 只认双引号。硬编码反引号会让
// 所有相关查询在 PostgreSQL 上直接报语法错误（选渠道主链路与全部配置读取都会失败），
// 因此统一交给方言的 QuoteTo 生成。
func quoteIdent(db *gorm.DB, name string) string {
	if db == nil || db.Dialector == nil {
		return "`" + name + "`"
	}
	var sb strings.Builder
	db.Dialector.QuoteTo(&sb, name)
	return sb.String()
}

// groupColumn 返回本方言下可安全使用的 abilities.group 列引用。
func groupColumn() string {
	return quoteIdent(DB, "group")
}
