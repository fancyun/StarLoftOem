package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
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

// StarLoftFvCallback 处理上游 StarLoft 平台的人脸核验结果推送（JSON，携带平台签名）。
// 平台按发起时传入的 notify_url 回推；业务号与本平台记录的 up_biz_id 一一对应。
// POST /v1/callback/starloft/fv
func (h *CallbackHandler) StarLoftFvCallback(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "读取请求体失败"})
		return
	}

	var payload struct {
		BizNo string `json:"biz_no"`
		Sign  string `json:"sign"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		log.Printf("StarLoft 人脸核验回调: 解析请求体失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误"})
		return
	}
	if payload.Sign == "" {
		log.Printf("StarLoft 人脸核验回调: 缺少 sign")
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 sign"})
		return
	}

	log.Printf("收到 StarLoft 人脸核验回调: biz_no=%s body_len=%d", payload.BizNo, len(bodyBytes))
	if err := h.authService.HandleUpstreamCallback(string(bodyBytes), payload.Sign); err != nil {
		log.Printf("StarLoft 人脸核验回调处理失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "回调处理失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
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

// StarLoftSmsReportCallback 处理上游 StarLoft 平台的短信回执推送（JSON，携带平台签名）。
// 按 message_sid 关联本地发送记录并落地回执状态；签名校验失败一律丢弃。
// POST /v1/callback/starloft/sms-report
func (h *CallbackHandler) StarLoftSmsReportCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)

	var push struct {
		MessageSid string `json:"message_sid"`
		Phone      string `json:"phone"`
		RespCode   string `json:"resp_code"`
		CodeDesc   string `json:"code_desc"`
		RespTime   string `json:"resp_time"`
		SequenceID string `json:"sequence_id"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("短信回执推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}

	verifier := h.rt.SmsPushVerifier()
	if verifier == nil || !verifier.VerifySmsReceiptSign(bodyBytes) {
		log.Printf("短信回执推送: 签名校验失败，丢弃 message_sid=%s", push.MessageSid)
		logstore.RecordSysCall("callback", "starloft-sms-report", push.MessageSid, 0, "",
			upstream.RedactPayload(bodyBytes), "signature verification failed", 0)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	if push.MessageSid == "" {
		log.Printf("短信回执推送: 缺少 message_sid，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "starloft-sms-report", push.MessageSid, 0, push.SequenceID,
		upstream.RedactPayload(bodyBytes),
		fmt.Sprintf("phone=%s respCode=%s codeDesc=%s respTime=%s", maskPhone(push.Phone), push.RespCode, push.CodeDesc, push.RespTime), 1)

	if h.smsService == nil {
		log.Printf("短信回执推送: 短信服务未配置，忽略 message_sid=%s", push.MessageSid)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}

	if _, err := h.smsService.HandleSmsReportPush(upstream.SmsReportItem{
		SequenceId: push.SequenceID,
		Phone:      push.Phone,
		RespTime:   push.RespTime,
		RespCode:   push.RespCode,
		CodeDesc:   push.CodeDesc,
	}, push.MessageSid); err != nil {
		log.Printf("短信回执推送落库失败: message_sid=%s, err=%v", push.MessageSid, err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// StarLoftSmsStatusCallback 处理上游 StarLoft 平台的签名/模板审核状态推送（JSON，携带平台签名）。
// biz_type=sign 时 record_id 为平台签名记录 ID（即本平台记录的 up_sign_id）；
// biz_type=template 时优先取 up_id（平台报备到的上游模板 ID），为空则回落 record_id。
// 平台状态归一为本地口径：签名 2-通过 / 3-驳回；模板 1-通过 / 2-驳回；其余（审核中）忽略不改写。
// POST /v1/callback/starloft/sms-status
func (h *CallbackHandler) StarLoftSmsStatusCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)

	var push struct {
		BizType  string `json:"biz_type"`
		RecordID int64  `json:"record_id"`
		UpID     string `json:"up_id"`
		Status   int    `json:"status"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("短信审核状态推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}

	verifier := h.rt.SmsPushVerifier()
	if verifier == nil || !verifier.VerifySmsStatusSign(bodyBytes) {
		log.Printf("短信审核状态推送: 签名校验失败，丢弃 biz_type=%s record_id=%d", push.BizType, push.RecordID)
		logstore.RecordSysCall("callback", "starloft-sms-status", "", 0, "",
			upstream.RedactPayload(bodyBytes), "signature verification failed", 0)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "starloft-sms-status", "", 0, push.UpID,
		upstream.RedactPayload(bodyBytes),
		fmt.Sprintf("biz_type=%s record_id=%d status=%d reason=%s", push.BizType, push.RecordID, push.Status, push.Reason), 1)

	if h.smsService == nil {
		log.Printf("短信审核状态推送: 短信服务未配置，忽略 biz_type=%s record_id=%d", push.BizType, push.RecordID)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}

	switch push.BizType {
	case "sign":
		// 平台签名状态 2-通过 / 3-驳回 → 本地口径 1-通过 / 3-驳回
		status := ""
		switch push.Status {
		case 2:
			status = "1"
		case 3:
			status = "3"
		}
		if status == "" {
			break
		}
		if _, err := h.smsService.HandleSignStatusPush(strconv.FormatInt(push.RecordID, 10), status, push.Reason); err != nil {
			log.Printf("签名状态推送落库失败: record_id=%d, err=%v", push.RecordID, err)
		}
	case "template":
		// 平台模板状态 1-通过 / 2-驳回 → 本地口径 1-通过 / 3-驳回
		status := ""
		switch push.Status {
		case 1:
			status = "1"
		case 2:
			status = "3"
		}
		if status == "" {
			break
		}
		upTemplateID := push.UpID
		if upTemplateID == "" {
			upTemplateID = strconv.FormatInt(push.RecordID, 10)
		}
		if _, err := h.smsService.HandleTemplateStatusPush(upTemplateID, status, push.Reason); err != nil {
			log.Printf("模板状态推送落库失败: up_template_id=%s, err=%v", upTemplateID, err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// StarLoftSmsReplyCallback 处理上游 StarLoft 平台的短信上行回复推送（JSON，携带平台签名）。
// 按 message_sid 关联发送记录归属用户并落库；发送时若提供了 notify_url 则主动推送下游。
// 签名校验失败一律丢弃。
// POST /v1/callback/starloft/sms-reply
func (h *CallbackHandler) StarLoftSmsReplyCallback(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)

	var push struct {
		MessageSid  string `json:"message_sid"`
		Phone       string `json:"phone"`
		ContentDown string `json:"content_down"`
		ContentUp   string `json:"content_up"`
		SequenceID  string `json:"sequence_id"`
		Timestamp   string `json:"timestamp"`
		Status      string `json:"status"`
		Tag         string `json:"tag"`
	}
	if err := json.Unmarshal(bodyBytes, &push); err != nil {
		log.Printf("短信回复推送: 解析 body 失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}

	verifier := h.rt.SmsPushVerifier()
	if verifier == nil || !verifier.VerifySmsReplySign(bodyBytes) {
		log.Printf("短信回复推送: 签名校验失败，丢弃 message_sid=%s", push.MessageSid)
		logstore.RecordSysCall("callback", "starloft-sms-reply", push.MessageSid, 0, "",
			upstream.RedactPayload(bodyBytes), "signature verification failed", 0)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	if push.MessageSid == "" {
		log.Printf("短信回复推送: 缺少 message_sid，忽略")
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	logstore.RecordSysCall("callback", "starloft-sms-reply", push.MessageSid, 0, push.SequenceID,
		upstream.RedactPayload(bodyBytes),
		fmt.Sprintf("phone=%s sequence_id=%s content_up=%s content_down=%s status=%s",
			maskPhone(push.Phone), push.SequenceID, push.ContentUp, push.ContentDown, push.Status), 1)

	if h.smsService == nil {
		log.Printf("短信回复推送: 短信服务未配置，忽略 message_sid=%s", push.MessageSid)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
		return
	}
	if _, err := h.smsService.HandleReplyPush(upstream.SmsReplyItem{
		TaskId:      push.MessageSid,
		Phone:       push.Phone,
		SequenceId:  push.SequenceID,
		ContentDown: push.ContentDown,
		ContentUp:   push.ContentUp,
		Timestamp:   push.Timestamp,
		Status:      push.Status,
		Tag:         push.Tag,
	}); err != nil {
		log.Printf("短信回复推送落库失败: message_sid=%s, err=%v", push.MessageSid, err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
