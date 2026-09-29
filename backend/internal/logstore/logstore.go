package logstore

import (
	"fmt"

	"oemrpa/internal/utils"
)

// RecordAccess 记录一条 HTTP 访问日志到 access.log。
// category 取值 console/admin/api；callback 类访问由回调处理逻辑单独写入 syscall.log。
// 单行记录方法/路径/状态码/耗时/身份/类别/IP（敏感路径 IP 已在调用方脱敏置空）。
func RecordAccess(category string, userID, adminID int64, method, path string, statusCode int, latencyMS int64, ip, serviceCode string) {
	if utils.AccessLogger == nil {
		return
	}
	identity := "-"
	switch category {
	case "admin":
		if adminID > 0 {
			identity = fmt.Sprintf("admin=%d", adminID)
		}
	case "console":
		if userID > 0 {
			identity = fmt.Sprintf("user=%d", userID)
		}
	default:
		if userID > 0 {
			identity = fmt.Sprintf("user=%d", userID)
		}
	}
	svc := ""
	if serviceCode != "" {
		svc = " service=" + serviceCode
	}
	ipField := ""
	if ip != "" {
		ipField = " ip=" + ip
	}
	utils.AccessLogger.Printf("category=%s %s [%s] %s - %d (%dms)%s%s",
		category, identity, method, path, statusCode, latencyMS, svc, ipField)
}

// RecordSysCall 记录一条系统对第三方服务的调用/回调日志到 syscall.log。
// direction: request-平台调用第三方，callback-第三方回调平台。
// 请求/响应内容由调用方脱敏后传入。
func RecordSysCall(direction, channel, bizNo string, orderID int64, refCode, reqData, respData string, status int) {
	if utils.SysCallLogger == nil {
		return
	}
	if direction != "request" && direction != "callback" {
		direction = "request"
	}
	utils.SysCallLogger.Printf("direction=%s channel=%s biz_no=%s order_id=%d ref_code=%s status=%d req=%s resp=%s",
		direction, channel, bizNo, orderID, refCode, status, reqData, respData)
}
