package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"
)

// Token 是对外暴露的 API Key。
type Token struct {
	Id     int    `json:"id" gorm:"primaryKey"`
	UserId int    `json:"user_id" gorm:"index"`
	Name   string `json:"name" gorm:"size:128"`
	// KeyHash 存储 key 的 sha256，明文仅创建时返回一次
	KeyHash string `json:"-" gorm:"uniqueIndex;size:64"`
	// KeyPrefix 用于前端展示（如 sk-abc...）
	KeyPrefix string `json:"key_prefix" gorm:"size:16"`
	Status    int    `json:"status" gorm:"default:1"`
	Group     string `json:"group" gorm:"size:64;default:'default'"`
	// Models 允许的模型白名单，逗号分隔，空表示不限
	Models string `json:"models" gorm:"type:text"`
	// 额度：Quota 总额度，UsedQuota 已用；Unlimited 为 true 时不限额。
	// 注意：不设 gorm default，否则创建限额令牌（Unlimited=false）时会被 GORM 当作未设置而回退默认值。
	Unlimited bool  `json:"unlimited"`
	Quota     int64 `json:"quota" gorm:"default:0"`
	UsedQuota int64 `json:"used_quota" gorm:"default:0"`
	ExpiredAt int64 `json:"expired_at" gorm:"default:0"` // 0 表示永不过期
	CreatedAt int64 `json:"created_at"`
}

const (
	TokenStatusEnabled  = 1
	TokenStatusDisabled = 2
)

// ErrTokenNotFound 令牌不存在。
var ErrTokenNotFound = errors.New("token not found")

// HashKey 返回 key 的 sha256 十六进制。
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// CreateToken 创建令牌，plainKey 为明文。
func CreateToken(t *Token, plainKey string) error {
	t.KeyHash = HashKey(plainKey)
	if len(plainKey) > 10 {
		t.KeyPrefix = plainKey[:10]
	} else {
		t.KeyPrefix = plainKey
	}
	t.CreatedAt = nowMilli()
	return DB.Create(t).Error
}

// GetTokenByKey 按明文 key 查询有效令牌。
func GetTokenByKey(plainKey string) (*Token, error) {
	var t Token
	err := DB.Where("key_hash = ?", HashKey(plainKey)).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// AllowModel 判断令牌是否允许使用某模型。
func (t *Token) AllowModel(model string) bool {
	list := splitComma(t.Models)
	if len(list) == 0 {
		return true
	}
	for _, m := range list {
		if m == model {
			return true
		}
	}
	return false
}

// ListTokens 返回某用户的令牌。
func ListTokens(userID int) ([]*Token, error) {
	var list []*Token
	err := DB.Where("user_id = ?", userID).Order("id desc").Find(&list).Error
	return list, err
}

// DeleteToken 删除令牌。
func DeleteToken(id, userID int) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Token{}).Error
}

// ErrQuotaInsufficient 额度不足。
var ErrQuotaInsufficient = errors.New("quota insufficient")

// PreConsumeQuota 预扣额度（请求前）。
//   - unlimited 令牌：直接累加 used_quota（用于统计），永不拒绝。
//   - 限额令牌：仅当 used_quota+amount <= quota 时才扣减，原子操作防并发超额。
//
// 额度不足返回 ErrQuotaInsufficient。amount<=0 时为空操作。
func PreConsumeQuota(id int, amount int64) error {
	if amount <= 0 {
		return nil
	}
	res := DB.Model(&Token{}).
		Where("id = ? AND (unlimited = ? OR used_quota + ? <= quota)", id, true, amount).
		UpdateColumn("used_quota", gorm.Expr("used_quota + ?", amount))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrQuotaInsufficient
	}
	return nil
}

// RefundQuota 退还额度（请求失败或结算少于预扣时）。
func RefundQuota(id int, amount int64) {
	if amount <= 0 {
		return
	}
	DB.Model(&Token{}).Where("id = ?", id).
		UpdateColumn("used_quota", gorm.Expr("CASE WHEN used_quota >= ? THEN used_quota - ? ELSE 0 END", amount, amount))
}

// SettleQuota 结算：将预扣额度调整为实际用量。
//   - actual > reserved：补扣差额；
//   - actual < reserved：退还差额；
//   - 相等：无操作。
//
// 限额令牌补扣差额时仍受 quota 约束，避免并发结算突破限额。
func SettleQuota(id int, reserved, actual int64) error {
	diff := actual - reserved
	switch {
	case diff > 0:
		res := DB.Model(&Token{}).
			Where("id = ? AND (unlimited = ? OR used_quota + ? <= quota)", id, true, diff).
			UpdateColumn("used_quota", gorm.Expr("used_quota + ?", diff))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrQuotaInsufficient
		}
	case diff < 0:
		RefundQuota(id, -diff)
	}
	return nil
}
