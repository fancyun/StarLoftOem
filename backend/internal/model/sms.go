package model

import "time"

// 短信模板类型（与上游联麓的通知/营销通道对应，决定发送时所走的应用与所扣的资源包类型）
const (
	SmsTemplateTypeVerify    = "verify"    // 验证码
	SmsTemplateTypeNotify    = "notify"    // 通知
	SmsTemplateTypeMarketing = "marketing" // 营销
)

// 短信模板状态（绑定公共签名的模板需先经平台审核，通过后才报备上游）
const (
	SmsTemplateStatusPending        = 0 // 已报备上游，等待上游审核
	SmsTemplateStatusApproved       = 1 // 已审核通过（可用于发送）
	SmsTemplateStatusRejected       = 2 // 已驳回
	SmsTemplateStatusPlatformReview = 3 // 待平台审核（绑定公共签名，尚未报备上游）
)

// SmsPackProduct 短信模板类型对应的资源包类型：营销模板扣营销资源包，其余（验证码/通知）扣通用短信资源包。
func SmsPackProduct(templateType string) string {
	if templateType == SmsTemplateTypeMarketing {
		return ProductSMSMarketing
	}
	return ServiceSMS
}

// SmsSign 短信签名申请（通道 + 资质材料 + 联麓报备扩展字段）
func (SmsSign) TableName() string { return SmsDB + ".sms_sign" }

type SmsSign struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64     `json:"user_id" gorm:"not null;index"`                    // 提交用户
	BizNo          string    `json:"biz_no" gorm:"size:20;not null;uniqueIndex"`       // 平台 20 位唯一业务号
	SignName       string    `json:"sign_name" gorm:"size:64;not null"`                // 签名内容
	SignType       int       `json:"sign_type" gorm:"type:tinyint;not null;default:1"` // 签名来源：1-本公司 2-他公司
	Label          int       `json:"label" gorm:"type:tinyint;not null;default:1"`     // 资质类型：1-营业执照 2-商标
	CreditCodeURL  string    `json:"credit_code_url" gorm:"size:512"`                  // 营业执照图片 URL
	IDCardFront    string    `json:"id_card_front" gorm:"size:512"`                    // 身份证正面 URL
	IDCardBack     string    `json:"id_card_back" gorm:"size:512"`                     // 身份证反面 URL
	Company        string    `json:"company" gorm:"size:128"`                          // 公司名称（他公司必填）
	LegalPerson    string    `json:"legal_person" gorm:"size:64"`                      // 法人姓名（他公司必填）
	CreditCode     string    `json:"credit_code" gorm:"size:64"`                       // 统一社会信用代码（他公司必填）
	CreditUserName string    `json:"credit_user_name" gorm:"size:64"`                  // 经办人姓名（他公司必填）
	IDCard         string    `json:"id_card" gorm:"size:64"`                           // 经办人身份证号（他公司必填）
	Phone          string    `json:"phone" gorm:"size:32"`                             // 经办人手机号（他公司必填）
	SxCommits      string    `json:"sx_commits" gorm:"size:512"`                       // 用户接收意愿承诺函 URL
	AuthLetter     string    `json:"auth_letter" gorm:"size:512"`                      // 短信签名授权书 URL
	Screenshot     string    `json:"screenshot" gorm:"size:512"`                       // 商标备案截图 URL（label=2 必填）
	Channel        string    `json:"channel" gorm:"size:32"`                           // 上游通道，提交通道
	UpSignID       string    `json:"up_sign_id" gorm:"size:32"`                        // 上游返回 SignId
	Status         int       `json:"status" gorm:"type:tinyint;not null;default:0"`    // 0-待审核 1-审核中 2-通过 3-驳回
	IsPublic       int       `json:"is_public" gorm:"type:tinyint;not null;default:0"` // 0-私有（仅提交账号可用）1-公共（所有账号可用）
	ResultMessage  string    `json:"result_message" gorm:"size:255"`                   // 审核结果/驳回原因
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// SmsTemplate 短信模板（本地存表待审核；上游模板报备/人工创建后回填 TemplateID）
// 绑定公共签名的模板先经平台审核（status=3），通过后才报备上游；自有签名模板提交即报备上游。
func (SmsTemplate) TableName() string { return SmsDB + ".sms_template" }

type SmsTemplate struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID          int64     `json:"user_id" gorm:"not null;index"`                 // 提交用户
	TemplateName    string    `json:"template_name" gorm:"size:64;not null"`         // 模板名称
	TemplateContent string    `json:"template_content" gorm:"type:text;not null"`    // 模板内容（含变量）
	TemplateType    string    `json:"template_type" gorm:"size:20;not null"`         // verify-验证码 notify-通知 marketing-营销
	SignID          int64     `json:"sign_id" gorm:"not null"`                       // 关联 sms_sign.id
	SignName        string    `json:"sign_name" gorm:"size:64"`                      // 关联签名内容（快照，便于展示）
	Channel         string    `json:"channel" gorm:"size:32"`                        // 所选上游通道标识
	Status          int       `json:"status" gorm:"type:tinyint;not null;default:0"` // 0-待上游审核 1-已审核 2-驳回 3-待平台审核
	TemplateID      string    `json:"template_id" gorm:"size:32"`                    // 上游模板 ID（报备/人工创建后回填）
	Reason          string    `json:"reason" gorm:"size:255"`                        // 驳回原因/说明
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// SmsReply 短信上行回复记录（回复推送落库 / 按日拉取回复落库，供下游查询）
func (SmsReply) TableName() string { return SmsDB + ".sms_reply" }

type SmsReply struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      int64     `json:"user_id" gorm:"not null;index"`          // 归属用户（按 taskId 关联发送记录）
	TaskID      string    `json:"task_id" gorm:"size:64"`                 // 上游任务 ID（关联发送记录 message_sid）
	Phone       string    `json:"phone" gorm:"size:32"`                   // 接收号码（回复来源）
	SequenceID  string    `json:"sequence_id" gorm:"size:64;uniqueIndex"` // 序列 ID（推送有，唯一）
	ContentDown string    `json:"content_down" gorm:"type:text"`          // 下行短信内容
	ContentUp   string    `json:"content_up" gorm:"type:text"`            // 回复内容
	RespTime    string    `json:"resp_time" gorm:"size:32"`               // 回复时间（拉取为日期时间，推送为时间戳）
	Status      string    `json:"status" gorm:"size:20"`                  // 推送状态
	Tag         string    `json:"tag" gorm:"size:64"`                     // 自定义标签（原样回传）
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// SmsSendRecord 短信发送记录（下游 API 每次发送落一条，含条数与金额）
func (SmsSendRecord) TableName() string { return SmsDB + ".sms_send_record" }

type SmsSendRecord struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64     `json:"user_id" gorm:"not null;index:idx_user_created,priority:1"` // 发送用户
	BizNo          string    `json:"biz_no" gorm:"size:20;not null"`                            // 平台 20 位唯一业务号
	TemplateID     string    `json:"template_id" gorm:"size:32"`                                // 模板 ID
	SignName       string    `json:"sign_name" gorm:"size:64"`                                  // 签名内容
	PhoneNumberSet string    `json:"phone_number_set" gorm:"type:text"`                         // 手机号数组 JSON
	PhoneCount     int       `json:"phone_count" gorm:"not null;default:1"`                     // 计费条数（号码数 × 单条分条数，按上游预扣费条数校正）
	SmsType        string    `json:"sms_type" gorm:"size:20"`                                   // 短信类型（取所发模板的类型：verify/notify/marketing）
	MessageSid     string    `json:"message_sid" gorm:"size:64"`                                // 上游 taskId
	Channel        string    `json:"channel" gorm:"size:32"`                                    // 实际发送的上游通道
	RequestID      string    `json:"request_id" gorm:"size:64"`                                 // 请求唯一 ID
	NotifyURL      string    `json:"notify_url,omitempty" gorm:"size:500"`                      // 下游回执主动推送地址（发送时传入）
	APIID          int64     `json:"api_id,omitempty" gorm:"not null;default:0;index"`          // 发起该发送的 API 密钥 ID（下游回调按此密钥签名；0=控制台发起）
	Status         int       `json:"status" gorm:"type:tinyint;not null;default:0"`             // 0-成功 1-失败
	FailMessage    string    `json:"fail_message" gorm:"size:255"`                              // 失败原因
	UnitPrice      float64   `json:"unit_price" gorm:"type:decimal(10,4);not null"`             // 单价（元/条）
	Amount         float64   `json:"amount" gorm:"type:decimal(12,4);not null"`                 // 实际扣费金额（余额支付：条数 × 单价；资源包扣量：0）
	PayType        int       `json:"pay_type" gorm:"type:tinyint;not null;default:0"`           // 0-资源包 1-余额
	PackCount      int       `json:"pack_count" gorm:"not null;default:0"`                      // 本次扣减的资源包条数（余额支付为 0）
	PackID         int64     `json:"pack_id,omitempty" gorm:"not null;default:0"`               // 本次扣减的资源包 ID（回执失败时按此退回条数）
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime;index:idx_user_created,priority:2"`
}
