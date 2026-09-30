package model

import (
	"database/sql"
	"time"
)

// KycPersonal 账户个人实名认证记录表（只存账户实名；下游 API 调用记录在 oem_fv.auth_record）
func (KycPersonal) TableName() string { return SysDB + ".kyc" }

type KycPersonal struct {
	ID            int64         `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID        int64         `json:"user_id" gorm:"not null;index"`                       // 用户ID
	AuthRecordID  sql.NullInt64 `json:"auth_record_id,omitempty" gorm:"index"`               // 关联认证记录ID（通过下单完成实名时关联；账户实名不关联记录）
	BizNo         string        `json:"biz_no" gorm:"size:50;not null;uniqueIndex"`          // 全平台唯一业务流水号（平台调用时随机生成）
	ReturnURL     string        `json:"return_url,omitempty" gorm:"size:500"`                // 认证完成后跳转的URL
	NotifyURL     string        `json:"notify_url,omitempty" gorm:"size:500"`                // 异步通知回调URL
	BizExtraData  string        `json:"biz_extra_data,omitempty" gorm:"type:text"`           // 额外业务数据
	UpToken       string        `json:"up_token,omitempty" gorm:"size:100"`                  // 上游返回的token
	UpBizID       string        `json:"up_biz_id,omitempty" gorm:"size:50;index"`            // 上游返回的biz_id
	UpRequestID   string        `json:"up_request_id,omitempty" gorm:"size:50"`              // 上游返回的request_id
	Name          string        `json:"name" gorm:"size:50;not null"`                        // 实名姓名
	IDCard        string        `json:"id_card" gorm:"size:18;not null"`                     // 身份证号
	Status        int           `json:"status" gorm:"type:tinyint;not null;default:0;index"` // 0-待认证 1-认证中 2-认证成功 3-认证失败 4-已更换
	ResultCode    string        `json:"result_code,omitempty" gorm:"size:20"`                // 上游认证结果码
	ResultMessage string        `json:"result_message,omitempty" gorm:"size:255"`            // 认证结果消息
	ResultData    string        `json:"result_data,omitempty" gorm:"type:text"`              // 认证结果完整数据（JSON）
	Cost          float64       `json:"cost" gorm:"-"`                                       // 本次认证消耗金额（账户实名免费，恒为0；仅用于列表展示）
	VerifiedAt    *time.Time    `json:"verified_at,omitempty"`                               // 认证通过时间
	CreatedAt     time.Time     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time     `json:"updated_at" gorm:"autoUpdateTime"`
}

// KybEnterprise 企业实名认证记录表（表 kyb；kyc 为个人实名，二者并存、互不冲突）
func (KybEnterprise) TableName() string { return SysDB + ".kyb" }

type KybEnterprise struct {
	ID               int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           int64      `json:"user_id" gorm:"not null;index"`                             // 用户ID
	BizNo            string     `json:"biz_no" gorm:"size:50;not null;uniqueIndex"`                // 全平台唯一业务流水号
	CompanyName      string     `json:"company_name" gorm:"size:100;not null"`                     // 企业名称
	CreditCode       string     `json:"credit_code" gorm:"size:20;not null"`                       // 统一社会信用代码
	LegalName        string     `json:"legal_name,omitempty" gorm:"size:50"`                       // 法人姓名（后台人工实名不填写）
	LegalIDCard      string     `json:"legal_id_card,omitempty" gorm:"size:18"`                    // 法人身份证号（后台人工实名不填写）
	Source           int        `json:"source" gorm:"type:tinyint;not null;default:0"`             // 来源：0-自助 1-后台人工 2-用户提交人工审核
	AdminID          int64      `json:"admin_id,omitempty" gorm:"index"`                           // 后台人工实名/人工审核操作的管理员ID
	FourFactorStatus int        `json:"four_factor_status" gorm:"type:tinyint;not null;default:0"` // 工商四要素核验：0-待核验 1-通过 2-未通过
	FourFactorData   string     `json:"four_factor_data,omitempty" gorm:"type:text"`               // 四要素核验结果数据（JSON）
	UpToken          string     `json:"up_token,omitempty" gorm:"size:100"`                        // 法人扫脸上游token
	UpBizID          string     `json:"up_biz_id,omitempty" gorm:"size:50;index"`                  // 法人扫脸上游biz_id
	UpRequestID      string     `json:"up_request_id,omitempty" gorm:"size:50"`                    // 法人扫脸上游request_id
	Status           int        `json:"status" gorm:"type:tinyint;not null;default:0;index"`       // 0-待四要素 1-待法人扫脸 2-通过 3-未通过 4-待人工审核
	ResultCode       string     `json:"result_code,omitempty" gorm:"size:20"`                      // 上游结果码
	ResultMessage    string     `json:"result_message,omitempty" gorm:"size:255"`                  // 结果消息
	ResultData       string     `json:"result_data,omitempty" gorm:"type:text"`                    // 结果完整数据（JSON）
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`                                     // 认证通过时间
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
