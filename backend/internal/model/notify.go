package model

import "time"

// 通知重试状态
const (
	NotifyPending   = 0 // 待推
	NotifySuccess   = 1 // 成功
	NotifyAbandoned = 2 // 已放弃（超最大重试次数）
)

// NotifyRecord 下游通知重试记录（通知下游失败时落库，定时任务补推）。
// 以「业务类型 + 业务单号」为复合主键（不使用自增 id）：同一业务单号只保留一行，
// 重新入队即重置重试；推送成功后直接删除该行（不保留成功历史，留痕见 syscall.log/business.log）。
func (NotifyRecord) TableName() string { return SysDB + ".notify_record" }

type NotifyRecord struct {
	BizType     string     `json:"biz_type" gorm:"size:32;not null;primaryKey"` // 业务类型：fv_result/sms_receipt/sms_reply/sms_status
	BizNo       string     `json:"biz_no" gorm:"size:64;not null;primaryKey"`   // 业务单号（无单号的对象用业务内唯一标识，如 tpl-{id}、回复序列号）
	RecordID    int64      `json:"record_id" gorm:"not null"`                   // 关联业务记录 ID（便于按表主键排障）
	UserID      int64      `json:"user_id" gorm:"not null;default:0"`
	TargetURL   string     `json:"target_url" gorm:"size:500;not null"`
	Payload     string     `json:"payload" gorm:"type:text"` // 推送报文 JSON（含签名）
	Status      int        `json:"status" gorm:"type:tinyint;not null;default:0;index:idx_status_next,priority:1"`
	FailTimes   int        `json:"fail_times" gorm:"not null;default:0"`
	LastError   string     `json:"last_error" gorm:"size:255"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty" gorm:"index:idx_status_next,priority:2"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
