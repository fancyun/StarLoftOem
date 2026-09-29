package handler

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/service"
)

// AdminSMSHandler 短信签名/模板后台人工审核接口
type AdminSMSHandler struct {
	sms *service.SmsChannelService
}

func NewAdminSMSHandler(sms *service.SmsChannelService) *AdminSMSHandler {
	return &AdminSMSHandler{sms: sms}
}

// ListSigns 后台签名审核列表（status=-1 全部，默认待审核 0）
// GET /admin/sms/signs?status=0&page=1&page_size=10
func (h *AdminSMSHandler) ListSigns(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListSignsForAdmin(status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// ReviewSign 后台审核签名：status=2 通过 / 3 驳回
// POST /admin/sms/signs/:id/review
func (h *AdminSMSHandler) ReviewSign(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}
	var req struct {
		Status int    `json:"status" binding:"required"` // 2-通过 3-驳回
		Reason string `json:"reason"`                    // 驳回原因
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != 2 && req.Status != 3) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "审核结果无效"})
		return
	}
	if req.Status == 3 && req.Reason == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "驳回需填写原因"})
		return
	}

	if err := h.sms.ReviewSign(id, req.Status, req.Reason); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// QuerySignStatus 后台手动查询签名审核状态：调上游（联麓）核对并回写本地（与用户端同款）
// POST /admin/sms/signs/:id/query
func (h *AdminSMSHandler) QuerySignStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}

	sign, err := h.sms.QuerySignStatusForAdmin(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": sign})
}

// UpdateSignResultMessage 后台修改签名审核结果/失败原因文本（不改变审核状态，留空即清空）
// PUT /admin/sms/signs/:id/result-message
func (h *AdminSMSHandler) UpdateSignResultMessage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}
	var req struct {
		ResultMessage string `json:"result_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请求参数无效"})
		return
	}
	message := strings.TrimSpace(req.ResultMessage)
	if utf8.RuneCountInString(message) > 255 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "审核结果最多 255 个字符"})
		return
	}

	if err := h.sms.UpdateSignResultMessage(id, message); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ListTemplates 后台模板审核列表（status=-1 全部，默认待审核 0）
// GET /admin/sms/templates?status=0&page=1&page_size=10
func (h *AdminSMSHandler) ListTemplates(c *gin.Context) {
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListTemplatesForAdmin(status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// ReviewTemplate 后台审核模板：status=1 通过 / 2 驳回
// POST /admin/sms/templates/:id/review
func (h *AdminSMSHandler) ReviewTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板 ID 无效"})
		return
	}
	var req struct {
		Status int    `json:"status" binding:"required"` // 1-通过 2-驳回
		Reason string `json:"reason"`                    // 驳回原因
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != 1 && req.Status != 2) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "审核结果无效"})
		return
	}
	if req.Status == 2 && req.Reason == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "驳回需填写原因"})
		return
	}

	if err := h.sms.ReviewTemplate(id, req.Status, req.Reason); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// QueryTemplateStatus 后台手动查询模板审核状态：调上游（联麓）核对并回写本地（与用户端同款）
// POST /admin/sms/templates/:id/query
func (h *AdminSMSHandler) QueryTemplateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板 ID 无效"})
		return
	}

	t, err := h.sms.QueryTemplateStatusForAdmin(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": t})
}

// ListSendRecords 后台全量发送记录（管理端，含用户手机号，日期筛选）
// GET /admin/sms/records?page=1&page_size=10&start_date=2026-09-01&end_date=2026-09-11
func (h *AdminSMSHandler) ListSendRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListSendRecordsForAdmin(
		c.DefaultQuery("start_date", ""), c.DefaultQuery("end_date", ""), page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// SMSStats 后台全量短信统计（管理端）
// GET /admin/sms/stats?days=7
func (h *AdminSMSHandler) SMSStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}

	stats, err := h.sms.SMSStatsForAdmin(days)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats})
}

// ListReplies 后台全量短信上行回复（管理端，含用户手机号）
// GET /admin/sms/replies?page=1&page_size=10
func (h *AdminSMSHandler) ListReplies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListRepliesForAdmin(page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// SetSignPublic 后台设置签名的公共标记（0-私有 1-公共）：仅改可见性，不动上游签名
// PUT /admin/sms/signs/:id/public
func (h *AdminSMSHandler) SetSignPublic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}
	var req struct {
		IsPublic int `json:"is_public"` // 0-私有 1-公共
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请求参数无效"})
		return
	}

	if err := h.sms.SetSignPublic(id, req.IsPublic); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
