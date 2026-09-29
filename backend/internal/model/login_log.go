package model

import "time"

// UserLoginLog 用户登录记录（系统库）
func (UserLoginLog) TableName() string { return SysDB + ".user_login_log" }

type UserLoginLog struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int64     `json:"user_id,omitempty" gorm:"index"`                        // 用户ID（失败时可能为空）
	Account    string    `json:"account" gorm:"size:100;not null;index"`                // 登录账号（手机号/用户名）
	LoginType  string    `json:"login_type" gorm:"size:20;not null;default:'password'"` // 登录方式：password-密码 sms_code-短信验证码 wechat-微信一键登录
	IP         string    `json:"ip" gorm:"size:50"`                                     // 登录IP
	UserAgent  string    `json:"user_agent,omitempty" gorm:"size:500"`                  // 浏览器UA
	Status     int       `json:"status" gorm:"type:tinyint;not null;default:1"`         // 1-成功 0-失败
	FailReason string    `json:"fail_reason,omitempty" gorm:"size:255"`                 // 失败原因
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// AdminLoginLog 管理员登录记录（系统库）
func (AdminLoginLog) TableName() string { return SysDB + ".admin_login_log" }

type AdminLoginLog struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	AdminID    int64     `json:"admin_id,omitempty" gorm:"index"`               // 管理员ID（失败时可能为空）
	Username   string    `json:"username" gorm:"size:50;not null;index"`        // 管理员账号
	IP         string    `json:"ip" gorm:"size:50"`                             // 登录IP
	UserAgent  string    `json:"user_agent,omitempty" gorm:"size:500"`          // 浏览器UA
	Status     int       `json:"status" gorm:"type:tinyint;not null;default:1"` // 1-成功 0-失败
	FailReason string    `json:"fail_reason,omitempty" gorm:"size:255"`         // 失败原因
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}
