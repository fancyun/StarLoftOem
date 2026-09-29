// Package audit 统一审计日志：把业务状态与资金变动按固定字段写入 business.log（同时输出标准输出）。
// 目的：任何关键状态变更（认证结果落地、扣费/退款、订单入账、签名模板回写等）都留可 join 的痕迹，
// 日后数据库出现异常时可据此复盘并回填。行格式固定：audit action=<动作> k=v k=v ...
package audit

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"oemrpa/internal/model"
)

// Log 输出一行审计日志到 business.log（同时进入标准输出）
func Log(action string, fields ...string) {
	var b strings.Builder
	b.WriteString("audit action=")
	b.WriteString(action)
	for _, f := range fields {
		if f == "" {
			continue
		}
		b.WriteByte(' ')
		b.WriteString(f)
	}
	log.Print(b.String())
}

// Fields 把键值对转成排序后的 k=v 字段列表（字段顺序稳定，便于 awk/grep 与人工比对）
func Fields(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

// KV 拼接单个 k=v 字段
func KV(key string, value interface{}) string {
	return fmt.Sprintf("%s=%v", key, value)
}

// AuthRecord 认证记录审计：动作 + 来源 + 记录可 join 字段 + 附加字段
func AuthRecord(action, source string, r *model.AuthRecord, extra ...string) {
	fields := append([]string{KV("source", source)}, Record(r)...)
	Log(action, append(fields, extra...)...)
}

// KycRecord 账户实名审计：动作 + 来源 + 记录可 join 字段 + 附加字段
func KycRecord(action, source string, k *model.KycPersonal, extra ...string) {
	fields := append([]string{KV("source", source)}, Kyc(k)...)
	Log(action, append(fields, extra...)...)
}

// KybRecord 企业实名审计：动作 + 来源 + 记录可 join 字段 + 附加字段
func KybRecord(action, source string, k *model.KybEnterprise, extra ...string) {
	fields := append([]string{KV("source", source)}, Kyb(k)...)
	Log(action, append(fields, extra...)...)
}

// PaymentOrder 支付订单审计：动作 + 来源 + 订单可 join 字段 + 附加字段
func PaymentOrder(action, source string, o *model.PaymentOrder, extra ...string) {
	fields := append([]string{KV("source", source)}, Payment(o)...)
	Log(action, append(fields, extra...)...)
}

// SmsSend 短信发送记录（oem_sms.sms_send_record）可 join 字段
func SmsSend(r *model.SmsSendRecord) []string {
	if r == nil {
		return nil
	}
	return []string{
		KV("sms_record_id", r.ID),
		KV("biz_no", r.BizNo),
		KV("user_id", r.UserID),
		KV("task_id", r.MessageSid),
		KV("template_id", r.TemplateID),
		KV("sign_name", r.SignName),
		KV("phone_count", r.PhoneCount),
		KV("status", r.Status),
		KV("amount", fmt.Sprintf("%.4f", r.Amount)),
		KV("pay_type", r.PayType),
		KV("pack_count", r.PackCount),
		KV("pack_id", r.PackID),
		KV("unit_price", fmt.Sprintf("%.4f", r.UnitPrice)),
	}
}

// SmsSendRecord 短信发送审计：动作 + 来源 + 记录可 join 字段 + 附加字段
func SmsSendRecord(action, source string, r *model.SmsSendRecord, extra ...string) {
	fields := append([]string{KV("source", source)}, SmsSend(r)...)
	Log(action, append(fields, extra...)...)
}

// Record 认证记录（oem_fv.auth_record）可 join 字段
func Record(r *model.AuthRecord) []string {
	if r == nil {
		return nil
	}
	return []string{
		KV("record_id", r.ID),
		KV("biz_no", r.BizNo),
		KV("user_id", r.UserID),
		KV("product", r.Product),
		KV("status", r.Status),
		KV("result_code", r.ResultCode),
		KV("up_biz_id", r.UpBizID),
		KV("up_query_count", r.UpQueryCount),
		KV("cost", fmt.Sprintf("%.2f", r.Cost)),
		KV("pay_type", r.PayType),
		KV("pack_count", r.PackCount),
		KV("pack_id", r.UserPackID),
		KV("is_refunded", r.IsRefunded),
	}
}

// Kyc 账户实名记录（oem_sys.kyc）可 join 字段
func Kyc(k *model.KycPersonal) []string {
	if k == nil {
		return nil
	}
	return []string{
		KV("kyc_id", k.ID),
		KV("biz_no", k.BizNo),
		KV("user_id", k.UserID),
		KV("status", k.Status),
		KV("result_code", k.ResultCode),
		KV("up_biz_id", k.UpBizID),
	}
}

// Kyb 企业实名记录（oem_sys.kyb）可 join 字段
func Kyb(k *model.KybEnterprise) []string {
	if k == nil {
		return nil
	}
	return []string{
		KV("kyb_id", k.ID),
		KV("biz_no", k.BizNo),
		KV("user_id", k.UserID),
		KV("status", k.Status),
		KV("four_factor_status", k.FourFactorStatus),
		KV("result_code", k.ResultCode),
		KV("up_biz_id", k.UpBizID),
	}
}

// Payment 支付订单（oem_sys.payment_order）可 join 字段
func Payment(o *model.PaymentOrder) []string {
	if o == nil {
		return nil
	}
	return []string{
		KV("pay_order_no", o.PayOrderNo),
		KV("order_id", o.ID),
		KV("user_id", o.UserID),
		KV("intent", o.Intent),
		KV("channel", o.Channel),
		KV("amount", fmt.Sprintf("%.2f", o.Amount)),
		KV("status", o.Status),
		KV("refund_status", o.RefundStatus),
		KV("refund_amount", fmt.Sprintf("%.2f", o.RefundAmount)),
		KV("balance_amount", fmt.Sprintf("%.2f", o.BalanceAmount)),
		KV("biz_no", o.BizNo),
		KV("channel_trade_no", o.ChannelTradeNo),
	}
}
