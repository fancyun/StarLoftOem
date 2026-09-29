package model

import "time"

// ============================================================
// 推广分佣模型
//
// 表分布（价格覆盖按产品分库，其余在系统库 oem_sys）：
//
//   PriceOverride / FvPriceOverride / SmsPriceOverride → price_override
//       价格覆盖（仅 scope_type=user 单用户定向定价）：账户实名两档在系统库，
//       fv_auth/fv_self 在人脸核验库，sms 在短信库
//   UserCommission → user_commission  用户型推广提成流水
//   StaffCommission → staff_commission 员工销售提成流水
//   PromotionWithdraw   → aff_withdraw     推广提现（仅用户型推广提现）
//
// 推广身份不再有独立主体表：推广码落在 user.aff_code / admin_user.aff_code，
// 归属关系落在 user.referrer_type + referrer_id；提成与提现按 referrer_type + referrer_id 归属。
//
// 已移除的能力（对应表与逻辑一并删除）：推广商主体表 affiliate（申请/审核/白标/停用）、
// 进货余额账户、三口径财务流水、下级用户子账户、进货等级与等级成本价、库存出库流水。
// ============================================================

// 推介方类型（referrer_type）：推广身份由「用户」或「员工」发起，计提逻辑完全一致
const (
	RefTypeReferrerUser  = "user"  // 用户（普通用户申请成为推广商）
	RefTypeReferrerStaff = "staff" // 员工（后台账号自动成为销售）
)

// 提成流水表（按推介方类型分表）：用户型推广商与员工销售的提成口径不同，流水分开存放
var (
	TableUserCommission  = SysDB + ".user_commission"  // 用户型推广商提成流水
	TableStaffCommission = SysDB + ".staff_commission" // 员工销售提成流水
)

// AffRewardWindowMonths 用户型推广商（aff）的提成归因窗口：被推广用户自注册起 1 个自然月内的消费才计提，
// 超期后该用户的消费不再产生提成；员工销售不受此限制（长期有效）。
const AffRewardWindowMonths = 1

// SettlementTable 按推介方类型返回提成流水表名（user→user_commission，staff→staff_commission）
func SettlementTable(referrerType string) string {
	if referrerType == RefTypeReferrerStaff {
		return TableStaffCommission
	}
	return TableUserCommission
}

// DefaultCommissionRate 推广提成比例兜底默认值（20%）：全局配置缺失或非法时两档产品均取此值
const DefaultCommissionRate = 0.2

// 提成产品档位（按一级产品分档配置比例）
const (
	CommissionProductFV  = ProductFV  // 人脸核验
	CommissionProductSMS = ServiceSMS // 短信服务
)

// 提成关联业务表类型（ref_type）：资源包购买时用于区分所属产品档位
const (
	RefTypeUserResourcePack    = "user_resource_pack"     // 人脸核验资源包
	RefTypeSmsUserResourcePack = "sms_user_resource_pack" // 短信资源包（独立表）
)

// CommissionProductFor 判定一笔提成所属的产品档位（人脸核验 fv / 短信 sms）：
//   - 按次计费按业务类型区分（短信发送 sms_send / 人脸核验 fv_auth）；
//   - 资源包购买按关联表类型区分（短信资源包为独立表）；
//   - 退款冲回虽会落到人脸核验档，但冲回金额取自原流水、不参与比例计算。
func CommissionProductFor(bizType, refType string) string {
	if bizType == CommissionBizSmsSend || refType == RefTypeSmsUserResourcePack {
		return CommissionProductSMS
	}
	return CommissionProductFV
}

// PriceOverride 价格覆盖（平台给单个用户定向定价，存系统库；仅承载账户实名两档）
// 解析优先级：用户级覆盖 → 平台产品库价（推广商不再拥有自主定价能力）。
// 人脸核验与短信的覆盖价已按产品拆到各自产品库（见 FvPriceOverride / SmsPriceOverride）。
func (PriceOverride) TableName() string { return SysDB + ".price_override" }

// FvPriceOverride 人脸核验库（oem_fv）中的价格覆盖表：结构同 PriceOverride，仅所在库不同，
// 承载 fv_auth / fv_self 的用户级定价；AutoMigrate 按本类型在 FV 库建表。
type FvPriceOverride PriceOverride

func (FvPriceOverride) TableName() string { return FvDB + ".price_override" }

// SmsPriceOverride 短信库（oem_sms）中的价格覆盖表：结构同 PriceOverride，
// 承载 sms 的用户级定价（按次单价与资源包售价）。
type SmsPriceOverride PriceOverride

func (SmsPriceOverride) TableName() string { return SmsDB + ".price_override" }

// 价格覆盖作用域（仅保留单用户定向定价）
const (
	PriceScopeUser = "user" // 指定用户（平台定向定价）
)

// 价格覆盖类型
const (
	PriceTypeUnit = "unit" // 按次/按条单价，target 为服务标识
	PriceTypePack = "pack" // 资源包售价，target 为「产品标识:资源包ID」（如 fv_auth:3 / sms:3 / sms_marketing:3）
)

type PriceOverride struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ScopeType string    `json:"scope_type" gorm:"size:16;not null;uniqueIndex:uk_price_scope,priority:1"`
	ScopeID   int64     `json:"scope_id" gorm:"not null;uniqueIndex:uk_price_scope,priority:2"`
	PriceType string    `json:"price_type" gorm:"size:16;not null;default:'unit';uniqueIndex:uk_price_scope,priority:3"`
	Target    string    `json:"target" gorm:"size:64;not null;uniqueIndex:uk_price_scope,priority:4"`
	Price     float64   `json:"price" gorm:"type:decimal(10,2);not null;default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// UserCommission 用户型推广商提成流水（oem_sys.user_commission）
// 下级消费/购买资源包时，按利润计提给直接上级；与员工销售提成分表（见 StaffCommission）。
func (UserCommission) TableName() string { return TableUserCommission }

// 提成业务类型（账户实名由平台承担成本、不进入推广提成，故无对应类型）
const (
	CommissionBizPackPurchase = "pack_purchase" // 下级购买资源包
	CommissionBizSmsSend      = "sms_send"      // 短信发送
	CommissionBizFvAuth       = "fv_auth"       // 人脸核验
	CommissionBizRefund       = "refund"        // 退款冲回
)

type UserCommission struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ReferrerType string    `json:"referrer_type" gorm:"size:8;not null;default:'';index:idx_user_settle_referrer,priority:1"` // 推介方类型：user→user.id，staff→admin_user.id
	ReferrerID   int64     `json:"referrer_id" gorm:"not null;default:0;index:idx_user_settle_referrer,priority:2"`           // 推介方 ID（收款方）
	UserID       int64     `json:"user_id" gorm:"not null;index"`                                                             // 提成来源用户 user_id（付款方）
	Amount       float64   `json:"amount" gorm:"type:decimal(10,2);not null;default:0"`                                       // 正=计提，负=退款冲回
	BizType      string    `json:"biz_type" gorm:"size:32;not null;default:''"`
	RefType      string    `json:"ref_type" gorm:"size:32;not null;default:''"`
	RefID        int64     `json:"ref_id" gorm:"not null;default:0"`
	Remark       string    `json:"remark" gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// StaffCommission 员工销售提成流水（oem_sys.staff_commission）
// 与用户型推广的 UserCommission（user_commission）分表存放：两者提成口径不同——
// 用户型推广仅有「被推广用户注册后 1 个自然月」的归因窗口，员工销售长期有效。
// 收款方为 referrer_type='staff' + admin_user.id。
func (StaffCommission) TableName() string { return TableStaffCommission }

type StaffCommission struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ReferrerType string    `json:"referrer_type" gorm:"size:8;not null;default:'';index:idx_staff_settle_referrer,priority:1"` // 推介方类型（恒为 staff）
	ReferrerID   int64     `json:"referrer_id" gorm:"not null;default:0;index:idx_staff_settle_referrer,priority:2"`           // 员工销售 admin_user.id（收款方）
	UserID       int64     `json:"user_id" gorm:"not null;index"`                                                              // 提成来源用户 user_id（付款方）
	Amount       float64   `json:"amount" gorm:"type:decimal(10,2);not null;default:0"`                                        // 正=计提，负=退款冲回
	BizType      string    `json:"biz_type" gorm:"size:32;not null;default:''"`
	RefType      string    `json:"ref_type" gorm:"size:32;not null;default:''"`
	RefID        int64     `json:"ref_id" gorm:"not null;default:0"`
	Remark       string    `json:"remark" gorm:"size:255;not null;default:''"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// ---------- 推广提现 ----------

// 推广提现申请状态
const (
	PromotionWithdrawPending  = 0 // 待审核（申请时已从可提现收益扣除）
	PromotionWithdrawApproved = 1 // 已通过（待打款）
	PromotionWithdrawPaid     = 2 // 已完成（已打款）
	PromotionWithdrawRejected = 3 // 已驳回（金额退回可提现收益）
)

// PromotionWithdrawFeeRate 推广提现手续费率（1%，仅线下渠道收取；提现到余额免手续费）
const PromotionWithdrawFeeRate = 0.01

// 提现方式
const (
	PromotionWithdrawChannelBalance = "balance" // 提现到平台余额：站内划转，即时到账、免手续费、无需收款信息
)

// PromotionWithdraw 推广提现申请（人工提现：申请 → 管理员审核 → 线下打款并标记完成）
func (PromotionWithdraw) TableName() string { return SysDB + ".aff_withdraw" }

type PromotionWithdraw struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	ReferrerType string     `json:"referrer_type" gorm:"size:8;not null;default:'user'"`        // 推介方类型（提现仅用户型推广）
	ReferrerID   int64      `json:"referrer_id" gorm:"not null;default:0;index"`                // 用户型推广者 user.id
	Channel      string     `json:"channel" gorm:"size:16;not null;default:'balance'"`          // 提现方式（balance-提现到余额）
	Amount       float64    `json:"amount" gorm:"type:decimal(12,2);not null"`                  // 申请金额
	FeeRate      float64    `json:"fee_rate" gorm:"type:decimal(6,4);not null;default:0.01"`    // 手续费率
	Fee          float64    `json:"fee" gorm:"type:decimal(12,2);not null;default:0"`           // 手续费
	ActualAmount float64    `json:"actual_amount" gorm:"type:decimal(12,2);not null;default:0"` // 实际打款额（申请额 − 手续费）
	Status       int        `json:"status" gorm:"type:tinyint;not null;default:0;index"`
	PayeeInfo    string     `json:"payee_info" gorm:"size:500;not null;default:''"` // 收款信息（户名 / 方式 / 账号）
	RejectReason string     `json:"reject_reason" gorm:"size:255;not null;default:''"`
	AdminID      int64      `json:"admin_id" gorm:"not null;default:0"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	PaidAt       *time.Time `json:"paid_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}
