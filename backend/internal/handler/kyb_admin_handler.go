package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/service"
)

// KybAdminHandler 企业实名（kyb）管理（管理后台）
type KybAdminHandler struct {
	authService *service.AuthService
}

func NewKybAdminHandler(authService *service.AuthService) *KybAdminHandler {
	return &KybAdminHandler{authService: authService}
}

// VerifyKyb 后台人工企业实名（=公户验证）：录入企业名称与统一社会信用代码即开通
func (h *KybAdminHandler) VerifyKyb(c *gin.Context) {
	adminID := c.GetInt64("user_id") // 管理员 JWT 与用户共用 user_id 上下文键

	var req struct {
		UserID      int64  `json:"user_id" binding:"required"`
		CompanyName string `json:"company_name" binding:"required"`
		CreditCode  string `json:"credit_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写用户、企业名称与统一社会信用代码"})
		return
	}

	if err := h.authService.AdminCreateKybManual(req.UserID, req.CompanyName, req.CreditCode, adminID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "企业实名开通失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "企业实名已开通"})
}

// ReviewKybManual 后台人工审核企业实名申请（通过为企业开通实名，驳回需填写原因）
func (h *KybAdminHandler) ReviewKybManual(c *gin.Context) {
	adminID := c.GetInt64("user_id") // 管理员 JWT 与用户共用 user_id 上下文键

	var req struct {
		ID     int64  `json:"id" binding:"required"`
		Action string `json:"action" binding:"required"` // approve-通过 / reject-驳回
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if req.Action != "approve" && req.Action != "reject" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "审核动作不合法"})
		return
	}

	if err := h.authService.ReviewKybManual(req.ID, req.Action == "approve", req.Reason, adminID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	if req.Action == "approve" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "企业实名已开通"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已驳回"})
}

// ListKybRecords 企业实名记录列表（分页，status=-1 全部）
func (h *KybAdminHandler) ListKybRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := h.authService.ListKybRecords(status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询企业实名记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// ListKycPersonalRecords 个人实名（kyc）记录列表（分页，status=-1 全部）
func (h *KybAdminHandler) ListKycPersonalRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	list, total, err := h.authService.ListKycPersonalRecords(status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询个人实名记录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"list":      list,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}
