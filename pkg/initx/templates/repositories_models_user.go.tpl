package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() {
	dbhelper.RegisterTable(&User{})
}

// User 用户表
type User struct {
	// 用户名，唯一索引
	Username string `gorm:"column:username;type:varchar(100);uniqueIndex" json:"username"`
	// 邮箱
	Email string `gorm:"column:email;type:varchar(255)" json:"email"`
	// 密码（加密后）
	Password string `gorm:"column:password;type:varchar(255)" json:"-"`
	// 昵称
	Nickname string `gorm:"column:nickname;type:varchar(100)" json:"nickname"`
	// 状态（active/inactive），默认 "active"
	Status string `gorm:"column:status;type:varchar(20);default:'active'" json:"status"`
}

func (User) TableName() string {
	return "user"
}




