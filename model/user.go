package model

import "golang.org/x/crypto/bcrypt"

// User 管理后台用户。
type User struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	Username     string `json:"username" gorm:"uniqueIndex;size:64"`
	PasswordHash string `json:"-" gorm:"size:128"`
	Role         int    `json:"role" gorm:"default:1"` // 1=admin
	CreatedAt    int64  `json:"created_at"`
}

const RoleAdmin = 1

// SetPassword 设置 bcrypt 密码哈希。
func (u *User) SetPassword(pw string) error {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(h)
	return nil
}

// CheckPassword 校验密码。
func (u *User) CheckPassword(pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pw)) == nil
}

// CountUsers 返回用户总数。
func CountUsers() (int64, error) {
	var n int64
	err := DB.Model(&User{}).Count(&n).Error
	return n, err
}

// GetUserByUsername 按用户名查询。
func GetUserByUsername(name string) (*User, error) {
	var u User
	err := DB.Where("username = ?", name).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser 创建用户。
func CreateUser(u *User) error {
	u.CreatedAt = nowMilli()
	return DB.Create(u).Error
}
