package model

import "time"

// ResourcePack 平台资源包定义
func (ResourcePack) TableName() string { return FvDB + ".resource_pack" }

type ResourcePack struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:100;not null"`                       // 资源包名称
	TotalCount  int       `json:"total_count" gorm:"not null"`                         // 认证次数
	Price       float64   `json:"price" gorm:"type:decimal(10,2);not null"`            // 售价（元）
	Status      int       `json:"status" gorm:"type:tinyint;not null;default:1;index"` // 状态：1-上架 0-下架
	Product     string    `json:"product,omitempty" gorm:"size:32;index"`              // 所属产品/服务标识（如 fv/fv_auth/fv_self/sms），不同产品（服务）的资源包互不通用
	Description string    `json:"description" gorm:"size:255"`                         // 描述
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// UserResourcePack 用户已购资源包
func (UserResourcePack) TableName() string { return FvDB + ".user_resource_pack" }

type UserResourcePack struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64     `json:"user_id" gorm:"not null;index;index:idx_user_status,priority:1"`
	TotalCount     int       `json:"total_count" gorm:"not null"`                              // 总次数（快照）
	RemainingCount int       `json:"remaining_count" gorm:"not null"`                          // 剩余次数
	Product        string    `json:"product" gorm:"size:32;index"`                             // 所属子产品/服务标识（快照，对应 resource_pack.product）
	Price          float64   `json:"price" gorm:"type:decimal(10,2);not null;default:0"`       // 购买价格（元，快照：实付金额）
	Status         int       `json:"status" gorm:"type:tinyint;not null;default:1;index:idx_user_status,priority:2"` // 1-有效 0-已耗尽/禁用
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// SmsResourcePack 短信资源包定义（独立于人脸核验库，存放于短信库）
// product 区分二级类型：sms-验证码/通知短信、sms_marketing-营销短信，两类包互不通用。
func (SmsResourcePack) TableName() string { return SmsDB + ".resource_pack" }

type SmsResourcePack struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:100;not null"`                       // 资源包名称
	TotalCount  int       `json:"total_count" gorm:"not null"`                         // 短信条数
	Price       float64   `json:"price" gorm:"type:decimal(10,2);not null"`            // 售价（元）
	Status      int       `json:"status" gorm:"type:tinyint;not null;default:1;index"` // 状态：1-上架 0-下架
	Product     string    `json:"product" gorm:"size:32;not null;default:'sms';index"` // 所属类型：sms-验证码/通知短信、sms_marketing-营销短信
	Description string    `json:"description" gorm:"size:255"`                         // 描述
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// SmsUserResourcePack 用户已购短信资源包（短信库）
// product 为购包时的类型快照（对应 resource_pack.product），扣费时按此与模板类型匹配。
func (SmsUserResourcePack) TableName() string { return SmsDB + ".user_resource_pack" }

type SmsUserResourcePack struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64     `json:"user_id" gorm:"not null;index;index:idx_user_status,priority:1"`
	TotalCount     int       `json:"total_count" gorm:"not null"`                              // 总条数（快照）
	RemainingCount int       `json:"remaining_count" gorm:"not null"`                          // 剩余条数
	Product        string    `json:"product" gorm:"size:32;not null;default:'sms';index"`      // 所属类型快照：sms-验证码/通知短信、sms_marketing-营销短信
	Price          float64   `json:"price" gorm:"type:decimal(10,2);not null;default:0"`       // 购买价格（元，快照：实付金额）
	Status         int       `json:"status" gorm:"type:tinyint;not null;default:1;index:idx_user_status,priority:2"` // 1-有效 0-已耗尽/禁用
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
