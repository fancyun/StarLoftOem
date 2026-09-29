package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"oemrpa/internal/audit"
	"oemrpa/internal/logstore"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/upstream"

	"github.com/gin-gonic/gin"
)

// CallbackHandler 回调处理器
type CallbackHandler struct {
	authService    *service.AuthService
	balanceService *service.BalanceService
	paymentRepo    *repository.PaymentOrderRepository
	smsService     *service.SmsChannelService
	rt             *runtime.Runtime
}

// NewCallbackHandler 创建回调处理器
func NewCallbackHandler(
	authService *service.AuthService,
	balanceService *service.BalanceService,
	paymentRepo *repository.PaymentOrderRepository,
	smsService *service.SmsChannelService,
	rt *runtime.Runtime,
) *CallbackHandler {
	return &CallbackHandler{
		authService:    authService,
		balanceService: balanceService,
		paymentRepo:    paymentRepo,
		smsService:     smsService,
		rt:             rt,
	}
}

func (h *CallbackHandler) alipay() *upstream.AlipayClient { return h.rt.Alipay() }

func (h *CallbackHandler) wechatPay() *upstream.WechatPayClient { return h.rt.WechatPay() }

// FinAuthCallback 处理 FinAuth 异步回调（notify_url）
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_will_plus_return_notify_url
// POST 请求，Content-Type: application/x-www-form-urlencoded
// 参数: data (JSON 字符串), sign (HMAC 签名)
func (h *CallbackHandler) FinAuthCallback(c *gin.Context) {
	// 读取原始请求体（脱敏记录：body 含签名与可能的人脸/证件数据，仅记录长度）
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	log.Printf("收到 FinAuth 回调: Content-Type=%s, BodyLen=%d", c.GetHeader("Content-Type"), len(bodyBytes))

	// 解析 form data（重置 body）
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ParseForm()
	data := c.PostForm("data")
	sign := c.PostForm("sign")

	if data == "" {
		log.Printf("FinAuth 回调: 缺少 data 参数")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 data 参数",
		})
		return
	}

	if sign == "" {
		log.Printf("FinAuth 回调: 缺少 sign 参数")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 sign 参数",
		})
		return
	}

	// 处理回调（data 可能含人脸图片等敏感数据，仅记录截断片段）
	log.Printf("FinAuth 回调: DataLen=%d, SignLen=%d, DataHead=%s", len(data), len(sign), truncateStr(data, 120))
	err := h.authService.HandleUpstreamCallback(data, sign)
	if err != nil {
		log.Printf("FinAuth 回调处理失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "回调处理失败",
		})
		return
	}

	// 成功响应（必须返回 200）
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// AlipayCallback 处理支付宝异步通知（notify_url，RSA2 验签）
// 支付宝要求返回纯文本 "success" 表示通知处理成功
func (h *CallbackHandler) AlipayCallback(c *gin.Context) {
	if h.alipay() == nil {
		log.Printf("支付宝回调: 支付宝支付未配置")
		c.String(http.StatusOK, "failure")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		log.Printf("支付宝回调: 解析表单失败: %v", err)
		c.String(http.StatusOK, "failure")
		return
	}

	params := make(map[string]string)
	for k, v := range c.Request.PostForm {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}

	outTradeNo := params["out_trade_no"]
	tradeNo := params["trade_no"]
	tradeStatus := params["trade_status"]
	log.Printf("收到支付宝回调: out_trade_no=%s, trade_no=%s, trade_status=%s", outTradeNo, tradeNo, tradeStatus)
	// 回调原文入 syscall.log（去掉签名，便于日后按 pay_order_no 复盘入账依据）
	logstore.RecordSysCall("callback", "alipay", "", 0, outTradeNo, redactFormParams(params),
		fmt.Sprintf("trade_status=%s trade_no=%s amount=%s", tradeStatus, tradeNo, params["total_amount"]),
		boolToInt(tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED"))

	// 验证签名
	if !h.alipay().VerifyNotify(params) {
		log.Printf("支付宝回调签名验证失败: out_trade_no=%s", outTradeNo)
		c.String(http.StatusOK, "failure")
		return
	}

	// 查询订单
	order, err := h.paymentRepo.GetOrderByPayOrderNo(outTradeNo)
	if err != nil {
		log.Printf("支付宝回调订单不存在: out_trade_no=%s, err=%v", outTradeNo, err)
		c.String(http.StatusOK, "failure")
		return
	}

	// 非支付成功状态，忽略（返回 success 停止支付宝重试）
	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		log.Printf("支付宝回调非成功状态: out_trade_no=%s, trade_status=%s", outTradeNo, tradeStatus)
		c.String(http.StatusOK, "success")
		return
	}

	// 已支付落地（幂等）：资源包发放 / 充值入账 / 已关闭订单冲入余额
	if err := h.settleNotifyPaid("支付宝", order, tradeNo); err != nil {
		log.Printf("支付宝回调落地失败: out_trade_no=%s, err=%v", outTradeNo, err)
		c.String(http.StatusOK, "failure")
		return
	}
	c.String(http.StatusOK, "success")
}

// settleNotifyPaid 渠道异步通知「已支付」落地（幂等）：
// 资源包订单事务内原子完成「标记已支付 + 发放资源包」；充值订单仅当待支付时入账；
// 订单已关闭但用户仍完成支付时，将金额冲入余额并将用途改写为充值。
// 返回错误表示未完成落地，调用方应回「失败」让渠道重试。
func (h *CallbackHandler) settleNotifyPaid(channelName string, order *model.PaymentOrder, channelTradeNo string) error {
	if order.Intent == "resource_pack" {
		if err := h.balanceService.SettleResourcePackPaid(order.ID, channelTradeNo); err != nil {
			return fmt.Errorf("资源包支付落地失败: %w", err)
		}
		log.Printf("%s回调处理成功: pay_order_no=%s, user_id=%d, amount=%.2f", channelName, order.PayOrderNo, order.UserID, order.Amount)
		audit.PaymentOrder("payment_settle", "callback", order, audit.KV("kind", "resource_pack"))
		return nil
	}

	changed, err := h.paymentRepo.MarkOrderPaidIfPending(order.ID, channelTradeNo)
	if err != nil {
		return fmt.Errorf("更新支付订单状态失败: %w", err)
	}
	if !changed {
		// 状态未被更新：可能已处理过，也可能订单已关闭但用户仍完成支付（金额冲入余额，用途改写为充值）
		if order.Status == 3 {
			if credited, cerr := h.balanceService.SettleLatePaidClosedOrder(order, channelTradeNo); cerr != nil {
				return fmt.Errorf("已关闭订单转充值入账失败: %w", cerr)
			} else if credited {
				log.Printf("%s回调：已关闭订单完成支付，金额冲入余额: pay_order_no=%s, amount=%.2f", channelName, order.PayOrderNo, order.Amount)
				audit.PaymentOrder("payment_settle", "callback", order, audit.KV("kind", "late_paid_closed_order"))
			}
		}
		// 已处理过，无需重复入账
		return nil
	}

	if err := h.balanceService.RechargeBalance(order.UserID, order.Amount, order.ID, order.Channel); err != nil {
		return fmt.Errorf("入账失败: %w", err)
	}
	log.Printf("%s回调处理成功: pay_order_no=%s, user_id=%d, amount=%.2f", channelName, order.PayOrderNo, order.UserID, order.Amount)
	audit.PaymentOrder("payment_settle", "callback", order, audit.KV("kind", "recharge"))
	return nil
}

// WechatCallback 处理微信支付异步通知（notify_url，API v3 验签 + 解密）
// 微信支付要求返回 {"code":"SUCCESS"} 表示通知处理成功
func (h *CallbackHandler) WechatCallback(c *gin.Context) {
	if h.wechatPay() == nil {
		log.Printf("微信支付回调: 微信支付未配置")
		c.JSON(http.StatusOK, gin.H{"code": "FAIL", "message": "微信支付未配置"})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("微信支付回调: 读取请求体失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "读取请求体失败"})
		return
	}

	// 提取验签头（header 名统一转小写）
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	payload, err := h.wechatPay().VerifyNotify(headers, bodyBytes)
	if err != nil {
		log.Printf("微信支付回调验签/解密失败: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "验签失败"})
		return
	}

	log.Printf("收到微信支付回调: out_trade_no=%s, transaction_id=%s, trade_state=%s", payload.OutTradeNo, payload.TransactionID, payload.TradeState)
	logstore.RecordSysCall("callback", "wechat", "", 0, payload.OutTradeNo,
		fmt.Sprintf("out_trade_no=%s transaction_id=%s trade_state=%s total=%d", payload.OutTradeNo, payload.TransactionID, payload.TradeState, payload.Amount.Total),
		fmt.Sprintf("trade_state=%s", payload.TradeState), boolToInt(payload.TradeState == "SUCCESS"))

	// 非支付成功状态，忽略（返回成功停止微信重试）
	if payload.TradeState != "SUCCESS" {
		log.Printf("微信支付回调非成功状态: out_trade_no=%s, trade_state=%s", payload.OutTradeNo, payload.TradeState)
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}

	// 查询订单
	order, err := h.paymentRepo.GetOrderByPayOrderNo(payload.OutTradeNo)
	if err != nil {
		log.Printf("微信支付回调订单不存在: out_trade_no=%s, err=%v", payload.OutTradeNo, err)
		c.JSON(http.StatusOK, gin.H{"code": "FAIL", "message": "订单不存在"})
		return
	}

	// 已支付落地（幂等）：资源包发放 / 充值入账 / 已关闭订单冲入余额
	if err := h.settleNotifyPaid("微信支付", order, payload.TransactionID); err != nil {
		log.Printf("微信支付回调落地失败: out_trade_no=%s, err=%v", payload.OutTradeNo, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "处理失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

// truncateStr 截断长字符串用于日志输出，避免敏感/大体积数据刷屏
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// boolToInt 布尔转日志状态值（1-成功 0-失败）
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// maskPhone 手机号脱敏（保留前 3 位与后 2 位），用于回调日志
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-2:]
}

// redactFormParams 表单回调参数转日志文本：去掉签名（sign/sign_type），键按字典序输出
func redactFormParams(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return truncateStr(strings.Join(parts, "&"), 1000)
}

// SmsReportCallback 处理联麓短信回执推送（对应平台「发送状态推送地址」）。
// 无签名验签（联麓推送不回带签名桶）；按 taskId 关联本地发送记录并落地回执状态。
// 推送成功判定：返回 HTTP 200 且 body 携带 llcode="0"，否则上游 30min 后循环重推（最多 5 次）。
func (h *CallbackHandler) SmsReportCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	log.Printf("收到短信回执推送: BodyLen=%d, BodyHead=%s", len(bodyBytes), truncateStr(string(bodyBytes), 160))

	var report struct {
		TaskId   string `json:"taskId"`
		SeqId    string `json:"sequenceId"`
		Phone    string `json:"phone"`
		RespTime string `json:"resptime"`
		RespCode string `json:"respCode"`
		CodeDesc string `json:"codeDesc"`
		Status   string `json:"status"`
		Message  string `json:"message"`
		Tag      string `json:"tag"`
	}
	if err := json.Unmarshal(bodyBytes, &report); err != nil {
		log.Printf("短信回执推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if report.TaskId == "" {
		log.Printf("短信回执推送: 缺少 taskId，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "sms-report", report.TaskId, 0, report.SeqId, "",
		fmt.Sprintf("phone=%s respCode=%s codeDesc=%s status=%s respTime=%s", maskPhone(report.Phone), report.RespCode, report.CodeDesc, report.Status, report.RespTime), 1)

	if h.smsService == nil {
		log.Printf("短信回执推送: 短信服务未配置，忽略 taskId=%s", report.TaskId)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}

	item := upstream.SmsReportItem{
		SequenceId: report.SeqId,
		Phone:      report.Phone,
		RespTime:   report.RespTime,
		RespCode:   report.RespCode,
		CodeDesc:   report.CodeDesc,
	}
	if _, err := h.smsService.HandleSmsReportPush(item, report.TaskId); err != nil {
		log.Printf("短信回执推送落库失败: taskId=%s, err=%v", report.TaskId, err)
		// 落库失败也返回 llcode=0 防重推（重推无意义）
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
}

// SmsSignStatusCallback 处理联麓签名状态推送（对应上游「签名状态推送地址」）。
// 无签名验签（联麓推送不回带签名）；按 id（上游 SignId）关联本地签名并落地审核状态，
// 未知 SignId 忽略但不报错（避免重推）。始终返回 HTTP 200。
// POST /v1/callback/sms-sign-status
func (h *CallbackHandler) SmsSignStatusCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	log.Printf("收到签名状态推送: BodyLen=%d, BodyHead=%s", len(bodyBytes), truncateStr(string(bodyBytes), 160))

	var push struct {
		ProductId    string `json:"productId"`
		Status       string `json:"status"` // 1-通过 3-驳回
		ID           string `json:"id"`     // 上游签名 ID（SignId）
		Title        string `json:"title"`
		Content      string `json:"content"`
		Type         string `json:"type"`
		CTime        string `json:"cTime"`
		RefuseReason string `json:"refuseReason"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("签名状态推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if push.ID == "" {
		log.Printf("签名状态推送: 缺少签名 ID，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "sms-sign-status", "", 0, push.ID, "",
		fmt.Sprintf("signId=%s status=%s title=%s refuseReason=%s cTime=%s", push.ID, push.Status, push.Title, push.RefuseReason, push.CTime), 1)
	if h.smsService == nil {
		log.Printf("签名状态推送: 短信服务未配置，忽略 signId=%s", push.ID)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if _, err := h.smsService.HandleSignStatusPush(push.ID, push.Status, push.RefuseReason); err != nil {
		log.Printf("签名状态推送落库失败: signId=%s, err=%v", push.ID, err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
}

// SmsTemplateStatusCallback 处理联麓模板状态推送（对应上游「模板状态推送地址」）。
// 无签名验签（联麓推送不回带签名）；按 id（上游模板 ID）关联本地模板并落地审核状态，
// 未知模板 ID 忽略但不报错（避免重推）。始终返回 HTTP 200。
// POST /v1/callback/sms-template-status
func (h *CallbackHandler) SmsTemplateStatusCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	log.Printf("收到模板状态推送: BodyLen=%d, BodyHead=%s", len(bodyBytes), truncateStr(string(bodyBytes), 160))

	var push struct {
		ProductId    string `json:"productId"`
		Status       string `json:"status"` // 1-通过 3-驳回
		ID           string `json:"id"`     // 上游模板 ID
		Title        string `json:"title"`
		Content      string `json:"content"`
		Type         string `json:"type"`
		CTime        string `json:"cTime"`
		RefuseReason string `json:"refuseReason"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("模板状态推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if push.ID == "" {
		log.Printf("模板状态推送: 缺少模板 ID，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "sms-template-status", "", 0, push.ID, "",
		fmt.Sprintf("templateId=%s status=%s title=%s refuseReason=%s cTime=%s", push.ID, push.Status, push.Title, push.RefuseReason, push.CTime), 1)
	if h.smsService == nil {
		log.Printf("模板状态推送: 短信服务未配置，忽略 templateId=%s", push.ID)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if _, err := h.smsService.HandleTemplateStatusPush(push.ID, push.Status, push.RefuseReason); err != nil {
		log.Printf("模板状态推送落库失败: templateId=%s, err=%v", push.ID, err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
}

// SmsReplyCallback 处理联麓短信回复推送（对应上游「短信回复推送地址」）。
// 无签名验签（联麓推送不回带签名）；按 taskId 关联发送记录归属用户并落库，
// 发送时若提供了 notify_url 则主动推送下游；未知 taskId 忽略但不报错（避免重推）。
// 始终返回 HTTP 200 + llcode=0（上游重推判定）。
// POST /v1/callback/sms-reply
func (h *CallbackHandler) SmsReplyCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	log.Printf("收到短信回复推送: BodyLen=%d, BodyHead=%s", len(bodyBytes), truncateStr(string(bodyBytes), 160))

	var push struct {
		TaskId      string `json:"taskId"`
		Phone       string `json:"phone"`
		SequenceId  string `json:"sequenceId"`
		ContentDown string `json:"contentDown"`
		ContentUp   string `json:"contentUp"`
		Timestamp   string `json:"timestamp"`
		Status      string `json:"status"`
		Tag         string `json:"tag"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("短信回复推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if push.TaskId == "" {
		log.Printf("短信回复推送: 缺少 taskId，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "sms-reply", push.TaskId, 0, push.SequenceId, "",
		fmt.Sprintf("phone=%s sequenceId=%s contentUp=%s contentDown=%s status=%s", maskPhone(push.Phone), push.SequenceId, push.ContentUp, push.ContentDown, push.Status), 1)
	if h.smsService == nil {
		log.Printf("短信回复推送: 短信服务未配置，忽略 taskId=%s", push.TaskId)
		c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
		return
	}
	if _, err := h.smsService.HandleReplyPush(upstream.SmsReplyItem{
		TaskId:      push.TaskId,
		Phone:       push.Phone,
		SequenceId:  push.SequenceId,
		ContentDown: push.ContentDown,
		ContentUp:   push.ContentUp,
		Timestamp:   push.Timestamp,
		Status:      push.Status,
		Tag:         push.Tag,
	}); err != nil {
		log.Printf("短信回复推送落库失败: taskId=%s, err=%v", push.TaskId, err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "llcode": "0", "message": "success"})
}
