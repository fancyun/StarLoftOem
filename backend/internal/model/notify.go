package model

import "time"

// 通知重试状态
const (
	NotifyPending   = 0 // 待推
	NotifySuccess   = 1 // 成功
	NotifyAbandoned = 2 // 已放弃（超最大重试次数）
)

// NotifyRecord 下游通知重试记录（通知下游失败时落库，定时任务补推）
func (NotifyRecord) TableName() string { return SysDB + ".notify_record" }

type NotifyRecord struct {
	ID          int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	BizType     string     `json:"biz_type" gorm:"size:32;not null"` // 业务类型：fv_result/sms_receipt/sms_reply
	RecordID    int64      `json:"record_id" gorm:"not null"`        // 关联业务记录 ID
	UserID      int64      `json:"user_id" gorm:"not null;default:0"`
	TargetURL   string     `json:"target_url" gorm:"size:500;not null"`
	Payload     string     `json:"payload" gorm:"type:text"` // 推送报文 JSON（含签名）
	Status      int        `json:"status" gorm:"type:tinyint;not null;default:0"`
	FailTimes   int        `json:"fail_times" gorm:"not null;default:0"`
	LastError   string     `json:"last_error" gorm:"size:255"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
