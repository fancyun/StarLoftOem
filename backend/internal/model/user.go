package model

import (
	"database/sql"
	"time"
)

// User 平台用户（使用 Web 前端的用户）
func (User) TableName() string { return SysDB + ".user" }

// 实名状态（User.RealnameStatus）：个人实名 kyc、企业实名 kyb 共用同一状态列
const (
	RealnameNone       = 0 // 未实名
	RealnamePersonal   = 1 // 个人实名（kyc）
	RealnameEnterprise = 2 // 企业实名（kyb）
)

type User struct {
	ID             int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	Phone          string         `json:"phone" gorm:"size:20;not null;uniqueIndex"`
	Username       string         `json:"username" gorm:"size:50;not null;uniqueIndex"`
	PasswordHash   string         `json:"-" gorm:"size:255;not null"`
	Balance        float64        `json:"balance" gorm:"type:decimal(10,2);not null;default:0"`
	RealnameStatus int            `json:"realname_status" gorm:"type:tinyint;not null;default:0"` // 实名状态：0-未实名 1-个人实名(kyc) 2-企业实名(kyb)
	VerifiedName   sql.NullString `json:"verified_name,omitempty" gorm:"size:100"`                // 实名主体名称（个人=姓名，企业=企业名称）
	VerifiedNumber sql.NullString `json:"verified_number,omitempty" gorm:"size:128"`              // 实名主体证件号（个人=身份证号，企业=统一社会信用代码；存储加密）
	Status         int            `json:"status" gorm:"type:tinyint;not null;default:1;index"`
	AffCode        sql.NullString `json:"aff_code,omitempty" gorm:"size:12;uniqueIndex:uk_user_aff_code"` // 推广码（12 位数字+小写字母，未开通推广为 NULL）
	ReferrerType   string         `json:"referrer_type" gorm:"size:8;not null;default:''"`                // 归属推介方类型：user-用户 staff-员工（空=平台直营）
	ReferrerID     int64          `json:"referrer_id" gorm:"not null;default:0"`                          // 归属推介方 ID（user→user.id，staff→admin_user.id；0=平台直营）
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	// 实名免费次数重置基准：剩余免费次数 = 免费上限(3) - (已发起核验次数 - 基准)；管理员重置时把基准设为当前已用次数。
	PersonalFreeBase   int       `json:"personal_free_base" gorm:"not null;default:0"`   // 个人实名免费次数基准偏移
	EnterpriseFreeBase int       `json:"enterprise_free_base" gorm:"not null;default:0"` // 企业实名免费次数基准偏移
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// AdminUser 管理员/员工账号（后台登录账号；员工与销售同为该表记录，按 permissions 逐项授权）
func (AdminUser) TableName() string { return SysDB + ".admin_user" }

type AdminUser struct {
	ID           int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	Username     string         `json:"username" gorm:"size:50;not null;uniqueIndex"`
	PasswordHash string         `json:"-" gorm:"size:255;not null"`
	Nickname     string         `json:"nickname" gorm:"size:50"`
	Status       int            `json:"status" gorm:"type:tinyint;not null;default:1"`
	AffCode      sql.NullString `json:"aff_code,omitempty" gorm:"size:12;uniqueIndex:uk_admin_aff_code"` // 员工推广码（12 位数字+小写字母，未生成时为 NULL）
	Permissions  string         `json:"-" gorm:"size:1024;not null;default:''"`                          // 权限码，逗号分隔，all=全部（对外经显式 DTO 转为数组）
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// ApiKey 平台 API 密钥（下游调用平台产品服务的鉴权凭据，存放于 sys 库）。
// 一个账号可创建多把密钥，各自独立配置权限。
func (ApiKey) TableName() string { return SysDB + ".api" }

type ApiKey struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int64     `json:"user_id" gorm:"not null;index"`
	Name       string    `json:"name" gorm:"size:64;not null;default:''"` // 密钥名称/备注（用户自取，便于区分多把密钥）
	APIKey     string    `json:"api_key" gorm:"size:64;not null;uniqueIndex"`
	APISecret  string    `json:"-" gorm:"size:160;not null"`                        // API Secret（存储加密）
	Permission string    `json:"permission" gorm:"size:255;not null;default:'all'"` // 权限范围：all-全部端点，或逗号分隔的端点标识（如 fv_auth,sms_send，见 model.APIEndpoints）
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// 认证记录状态（auth_record.status）
const (
	AuthStatusPending    = 0 // 待认证：已建单，尚未取到上游 token
	AuthStatusProcessing = 1 // 认证中：已取到 token，等待用户完成核身
	AuthStatusSuccess    = 2 // 认证成功（计费）
	AuthStatusFailed     = 3 // 认证失败：结果码决定计费/不计费（6000/6100 等不计费并退款）
	AuthStatusCanceled   = 4 // 已取消
	AuthStatusTimeout    = 5 // 超时结束：核身链接过期仍未完成，不计费，已扣费用原路退还
	AuthStatusStartFail  = 6 // 发起失败：上游拦截/连接超时，未产生扣费，不涉及退款
)

// 认证结果码（auth_record.result_code，终态且非上游结果码时填写）
const (
	ResultCodeTimeout       = "TIMEOUT"        // 发起阶段上游连接超时/被拦截（未扣费）
	ResultCodeExpired       = "EXPIRED"        // 核身链接超时未完成（不计费，已扣费用退还）
	ResultCodeDataDestroyed = "DATA_DESTROYED" // 上游认证结果已不可取回（查询次数/有效期用尽，不计费，已扣费用退还）
)

// UpstreamQueryLimit 上游 get_result 接口对同一 biz_id 的最大可查询次数（硬限制，不可突破）。
// 上游文档：「该接口的调用信息保存有效期为一天，且仅支持 3 次调用，第 4 次或超过有效期调用则会返回错误信息」，
// 对应错误码 403 DATA_DESTROYED（超过可查询时间或超过最多可查询次数）。
// 一旦超限，该笔核身结果永久不可取回，因此每单查询必须严格按此预算，只在必要链路（用户回跳校对）消耗。
const UpstreamQueryLimit = 3

// AuthRecord 认证记录
func (AuthRecord) TableName() string { return FvDB + ".auth_record" }

type AuthRecord struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	BizNo          string     `json:"biz_no" gorm:"size:50;not null;uniqueIndex"` // 全平台唯一业务流水号（平台调用时随机生成）
	UserID         int64      `json:"user_id" gorm:"not null;index"`
	Name           string     `json:"name,omitempty" gorm:"size:50;not null;default:''"`       // 实名姓名（下游 API 调用时记录）
	IDCard         string     `json:"id_card,omitempty" gorm:"size:18;not null;default:''"`    // 实名身份证号（下游 API 调用时记录）
	UserPhone      string     `json:"user_phone,omitempty" gorm:"-"`                           // 联表查询时的用户手机号（管理后台订单列表用）
	ReturnURL      string     `json:"return_url,omitempty" gorm:"size:500"`                    // 认证完成后跳转的URL
	NotifyURL      string     `json:"notify_url,omitempty" gorm:"size:500"`                    // 异步通知回调URL
	BizExtraData   string     `json:"biz_extra_data,omitempty" gorm:"type:text"`               // 额外业务数据
	UpToken        string     `json:"up_token,omitempty" gorm:"size:100"`                      // 上游返回的token
	UpBizID        string     `json:"up_biz_id,omitempty" gorm:"size:50"`                      // 上游返回的biz_id
	UpRequestID    string     `json:"up_request_id,omitempty" gorm:"size:50"`                  // 上游返回的request_id
	UpQueryCount   int        `json:"up_query_count" gorm:"not null;default:0"`                // 已向上游 get_result 查询次数（硬上限见 UpstreamQueryLimit）
	TokenExpireAt  *time.Time `json:"token_expire_at,omitempty"`                               // 上游 token 到期时间（get_token 返回的 expired_time，可为 NULL）
	ResultCode     string     `json:"result_code,omitempty" gorm:"size:20"`                    // 认证结果码
	ResultMessage  string     `json:"result_message,omitempty" gorm:"size:255"`                // 认证结果消息
	ResultData     string     `json:"result_data,omitempty" gorm:"type:text"`                  // 认证结果完整数据（JSON）
	Status         int        `json:"status" gorm:"type:tinyint;not null;default:0;index"`     // 0-待认证 1-认证中 2-已完成 3-失败 4-已取消 5-超时结束（不计费已退款） 6-发起失败（未扣费），见 AuthStatus*
	Cost           float64    `json:"cost" gorm:"type:decimal(10,2);not null;default:0"`       // 实际扣费金额（余额支付；资源包扣量为 0，未扣费失败也为 0）
	PayType        int        `json:"pay_type" gorm:"type:tinyint;not null;default:0"`         // 扣费方式：0-免费 1-余额 2-资源包
	UserPackID     int64      `json:"user_pack_id,omitempty"`                                  // 使用的用户资源包ID（pay_type=2 时）
	PackCount      int        `json:"pack_count" gorm:"not null;default:0"`                    // 本次扣减的资源包次数（余额/免费为 0）
	Product        string     `json:"product,omitempty" gorm:"size:32;index"`                  // 所属产品/服务标识（如 fv/fv_auth/fv_self/sms），按此匹配相应用途的资源包
	APIID          int64      `json:"api_id,omitempty" gorm:"not null;default:0;index"`        // 发起该订单的 API 密钥 ID（下游回调按此密钥签名；0=非 API 发起）
	IsRefunded     int        `json:"is_refunded" gorm:"type:tinyint;not null;default:0"`      // 超时是否已退款：0-否 1-是
	NotifyTimes    int        `json:"notify_times" gorm:"not null;default:0"`                  // 回调用户次数
	NotifyStatus   int        `json:"notify_status" gorm:"type:tinyint;not null;default:0"`    // 回调用户状态：0-待通知 1-通知成功 2-通知失败
	BestImgFetched int        `json:"best_img_fetched" gorm:"type:tinyint;not null;default:0"` // 活体最佳图是否已领取：0-未领取 1-已领取（仅标记，图片不落库）
	MediaDir       string     `json:"media_dir,omitempty" gorm:"size:255;not null;default:''"` // 认证照片/视频本地保存目录（相对 UploadDir）
	MediaExpireAt  *time.Time `json:"media_expire_at,omitempty"`                               // 认证照片/视频过期时间（完成时间+30天，过后清理）
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"` // 完成时间
}

// PaymentOrder 支付订单（充值订单）
func (PaymentOrder) TableName() string { return SysDB + ".payment_order" }

// 支付渠道
const (
	ChannelAlipay = "alipay" // 支付宝
	ChannelWechat = "wechat" // 微信支付
	ChannelManual = "manual" // 人工支付
)

type PaymentOrder struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	PayOrderNo     string     `json:"pay_order_no" gorm:"size:50;not null;uniqueIndex"` // 支付流水号
	UserID         int64      `json:"user_id" gorm:"not null;index"`
	UserPhone      string     `json:"user_phone,omitempty" gorm:"-"`                           // 联表查询时的用户手机号（管理后台支付记录用）
	Amount         float64    `json:"amount" gorm:"type:decimal(10,2);not null"`               // 充值金额（元）
	Channel        string     `json:"channel" gorm:"size:20;not null"`                         // 支付渠道：alipay-支付宝 wechat-微信 manual-人工支付
	ChannelTradeNo string     `json:"channel_trade_no,omitempty" gorm:"size:100"`              // 渠道交易号（支付宝 trade_no）
	BankSerialNo   string     `json:"bank_serial_no,omitempty" gorm:"size:100;index"`          // 银行流水单号（人工支付唯一账单键：账号_记账时间_交易流水号）
	Status         int        `json:"status" gorm:"type:tinyint;not null;default:0;index"`     // 0-待支付 1-已支付 2-已退款 3-已关闭
	RefundStatus   int        `json:"refund_status" gorm:"type:tinyint;not null;default:0"`    // 退款状态：0-未退款 1-部分退款 2-全额退款
	RefundAmount   float64    `json:"refund_amount" gorm:"type:decimal(10,2);default:0"`       // 退款金额
	ExpireTime     *time.Time `json:"expire_time,omitempty"`                                   // 过期时间
	PaidAt         *time.Time `json:"paid_at,omitempty"`                                       // 支付时间
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`                                   // 退款时间
	Intent         string     `json:"intent" gorm:"size:20;not null;default:'recharge';index"` // 支付用途：recharge-余额充值 resource_pack-购买资源包
	BizNo          string     `json:"biz_no,omitempty" gorm:"size:50;index"`                   // 关联业务单号（购买资源包时为资源包ID）
	BalanceAmount  float64    `json:"balance_amount" gorm:"type:decimal(10,2);default:0"`      // 组合支付中的余额支付部分
	StockReserved  int        `json:"stock_reserved" gorm:"type:tinyint;not null;default:0"`   // 历史字段：资源包占库存标记（当前业务无库存概念，恒为 0）
	PayInfo        string     `json:"pay_info,omitempty" gorm:"type:text"`                     // 渠道支付信息 JSON（支付宝 pay_url/微信 code_url/h5_url），供复用待支付单时返回
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
