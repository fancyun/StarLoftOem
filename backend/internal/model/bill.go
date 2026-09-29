package model

import "time"

// 账单类型常量
const (
	BillTypeRecharge = 1 // 充值
	BillTypeConsume  = 2 // 消费
	BillTypeRefund   = 3 // 退款
)

// 花费类型（同一产品下区分不同花费来源）
const (
	SpendTypeRecharge         = "recharge"      // 充值入账
	SpendTypeBalance          = "balance"       // 余额直接扣费（未走资源包）
	SpendTypePackPurchase     = "pack_purchase" // 购买资源包
	SpendTypeRefund           = "refund"        // 退款（余额退回/资源包次数退回）
)

// 计费方式
const (
	PayTypePack    = 0 // 资源包
	PayTypeBalance = 1 // 余额
	PayTypeAlipay  = 2 // 支付宝
	PayTypeWechat  = 3 // 微信支付
)

// PayTypeOfChannel 支付渠道对应的计费方式（未知渠道回退支付宝，保持历史记账口径）
func PayTypeOfChannel(channel string) int {
	switch channel {
	case ChannelWechat:
		return PayTypeWechat
	default:
		return PayTypeAlipay
	}
}

// Bill 统一账单（系统库）：记录全平台资金/消耗明细，按产品+花费类型归类，
// 通过 RefType/RefID 绑定业务表（auth_record / sms_send_record / payment_order / user_resource_pack），
// 通过 PayOrderID 绑定支付单，BalanceBefore/After 记录余额变动快照（非余额变动时前后相等）。
type Bill struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	BizNo         string    `json:"biz_no" gorm:"size:20;not null;uniqueIndex"`        // 平台 20 位唯一业务号
	UserID        int64     `json:"user_id" gorm:"not null;index"`                     // 归属用户
	Product       string    `json:"product" gorm:"size:32;index"`                      // 产品：fv/sms（充值/退款可能为空）
	Service       string    `json:"service" gorm:"size:32;index"`                      // 服务：fv_auth/fv_self/sms/kyc_personal/kyc_enterprise/recharge
	BillType      int       `json:"bill_type" gorm:"type:tinyint;not null;index"`      // 1-充值 2-消费 3-退款
	SpendType     string    `json:"spend_type" gorm:"size:24;not null;index"`          // 花费类型：recharge/balance/pack_purchase/refund（资源包扣量不入本表）
	PayType       int       `json:"pay_type" gorm:"type:tinyint;not null"`             // 计费方式：0-资源包 1-余额 2-支付宝 3-微信
	Amount        float64   `json:"amount" gorm:"type:decimal(12,4);not null"`         // 金额
	BalanceBefore float64   `json:"balance_before" gorm:"type:decimal(12,4);not null"` // 变动前余额
	BalanceAfter  float64   `json:"balance_after" gorm:"type:decimal(12,4);not null"`  // 变动后余额
	RefType       string    `json:"ref_type" gorm:"size:24;index"`                     // 绑定业务表类型：auth_record/sms_send_record/payment_order/user_resource_pack/resource_pack
	RefID         int64     `json:"ref_id" gorm:"index"`                               // 绑定业务记录 ID
	RefBizNo      string    `json:"ref_biz_no" gorm:"size:20"`                         // 绑定业务单号（可读，如订单 biz_no）
	PayOrderID    int64     `json:"pay_order_id" gorm:"index"`                         // 绑定支付单 ID（payment_order.id，充值/购买资源包时）
	BankSerialNo  string    `json:"bank_serial_no,omitempty" gorm:"-"`                 // 银行流水单号：取自关联支付单（payment_order.bank_serial_no），非本表列
	Remark        string    `json:"remark" gorm:"size:255"`                            // 备注
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

func (Bill) TableName() string { return SysDB + ".bill" }
