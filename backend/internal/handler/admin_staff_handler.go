package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"oemrpa/internal/config"
	"oemrpa/internal/middleware"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/service"
	"oemrpa/internal/utils"
)

// AdminStaffHandler 后台账号（员工/管理员）管理：新增、改权限、启停、重置密码。
// 员工与销售均为后台账号，创建时逐项勾选权限；创建成功后自动获得销售推广身份（所有员工自动是销售）。
type AdminStaffHandler struct {
	adminRepo        *repository.AdminRepository
	promotionService *service.PromotionService
}

func NewAdminStaffHandler(adminRepo *repository.AdminRepository, promotionService *service.PromotionService) *AdminStaffHandler {
	return &AdminStaffHandler{adminRepo: adminRepo, promotionService: promotionService}
}

// adminUserView 后台账号视图（绝不返回密码哈希）
type adminUserView struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Status      int        `json:"status"`
	Permissions []string   `json:"permissions"`
	IsSuper     bool       `json:"is_super"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// failStaff 统一错误响应（与平台既有约定一致：HTTP 200 + 业务 code）
func failStaff(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "message": message})
}

// withinOperatorScope 目标权限串是否全部落在操作者权限范围内（操作者持 all 时恒为真）。
// 用于防止持 sys.admins.write 的普通员工通过新增账号 / 改权限 / 重置他人密码，
// 管理（或创建出）权限高于自身的账号，从而提权。
func withinOperatorScope(c *gin.Context, perms string) bool {
	operator := middleware.AdminPermissions(c)
	for _, code := range config.ParsePermissions(perms) {
		if !config.HasPermission(operator, code) {
			return false
		}
	}
	return true
}

// logStaffOperation 记录后台账号操作日志（与 AdminHandler.logAdminOperation 同格式）
func logStaffOperation(c *gin.Context, operation, resourceType string, resourceID int64, details string) {
	utils.AdminLogger.Printf("admin_id=%d operation=%s resource_type=%s resource_id=%d ip=%s details=%s",
		c.GetInt64("admin_id"), operation, resourceType, resourceID, c.ClientIP(), details)
}

// toAdminView 转换为对外视图：权限串 → 数组（`all` 或分组通配码按原样返回，前端据此渲染）
func toAdminView(a *model.AdminUser) adminUserView {
	return adminUserView{
		ID:          a.ID,
		Username:    a.Username,
		Nickname:    a.Nickname,
		Status:      a.Status,
		Permissions: config.ParsePermissions(a.Permissions),
		IsSuper:     config.HasPermission(a.Permissions, model.PermissionAll),
		LastLoginAt: a.LastLoginAt,
		CreatedAt:   a.CreatedAt,
	}
}

// Me 当前登录账号信息（含权限清单，供前端刷新后重建菜单与路由守卫）
// GET /admin/me
func (h *AdminStaffHandler) Me(c *gin.Context) {
	admin, err := h.adminRepo.GetAdminByID(c.GetInt64("admin_id"))
	if err != nil || admin == nil {
		failStaff(c, 401, "账号不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": toAdminView(admin)})
}

// List 后台账号列表
// GET /admin/admins
func (h *AdminStaffHandler) List(c *gin.Context) {
	page, pageSize := paginationParams(c)
	list, total, err := h.adminRepo.ListAdmins(page, pageSize)
	if err != nil {
		log.Printf("查询后台账号列表失败: %v", err)
		failStaff(c, 500, "查询失败")
		return
	}
	views := make([]adminUserView, 0, len(list))
	for _, a := range list {
		views = append(views, toAdminView(a))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"list": views, "total": total, "page": page, "page_size": pageSize},
	})
}

// Create 新增后台账号（员工/管理员），并自动创建其销售推广身份
// POST /admin/admins
func (h *AdminStaffHandler) Create(c *gin.Context) {
	if !middleware.HasPermission(c, model.WritePermission(model.PermissionSysAdmins)) {
		failStaff(c, 403, "无权限新增账号")
		return
	}
	var req struct {
		Username    string   `json:"username" binding:"required"`
		Password    string   `json:"password" binding:"required"`
		Nickname    string   `json:"nickname"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failStaff(c, 400, "invalid request parameters")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if len(req.Username) < 3 {
		failStaff(c, 400, "用户名至少 3 位")
		return
	}
	if len(req.Password) < 6 {
		failStaff(c, 400, "密码至少 6 位")
		return
	}
	if existing, err := h.adminRepo.GetAdminByUsername(req.Username); err == nil && existing != nil {
		failStaff(c, 400, "用户名已存在")
		return
	}

	// 授予的权限不得超出操作者自身范围，防止普通员工创建高权限账号后提权
	normalizedPerms := config.NormalizePermissions(req.Permissions)
	if !withinOperatorScope(c, normalizedPerms) {
		failStaff(c, 403, "不能授予超出自身权限范围外的权限")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		failStaff(c, 500, "创建失败")
		return
	}
	a := &model.AdminUser{
		Username:     req.Username,
		PasswordHash: string(hash),
		Nickname:     req.Nickname,
		Status:       1,
		Permissions:  normalizedPerms,
	}
	if err := h.adminRepo.CreateAdmin(a); err != nil {
		log.Printf("创建后台账号失败: %v", err)
		failStaff(c, 500, "创建失败（用户名可能已存在）")
		return
	}

	// 所有员工自动成为销售：创建推广码（失败仅记日志，不回滚账号）
	if _, err := h.promotionService.EnsureStaffAffCode(a.ID); err != nil {
		log.Printf("为员工创建销售推广码失败 [admin=%d]: %v", a.ID, err)
	}

	logStaffOperation(c, "admin_create", "admin_user", a.ID,
		fmt.Sprintf("username=%s permissions=%s", a.Username, a.Permissions))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功", "data": toAdminView(a)})
}

// Update 修改账号昵称/状态/权限
// PUT /admin/admins/:id
func (h *AdminStaffHandler) Update(c *gin.Context) {
	if !middleware.HasPermission(c, model.WritePermission(model.PermissionSysAdmins)) {
		failStaff(c, 403, "无权限修改账号")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		failStaff(c, 400, "invalid id")
		return
	}
	target, err := h.adminRepo.GetAdminByID(id)
	if err != nil || target == nil {
		failStaff(c, 404, "账号不存在")
		return
	}
	var req struct {
		Nickname    *string   `json:"nickname"`
		Status      *int      `json:"status"`
		Permissions *[]string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failStaff(c, 400, "invalid request parameters")
		return
	}
	operatorID := c.GetInt64("admin_id")

	// 不能管理权限高于自身的账号（否则可重置高权限账号密码后登录提权）
	if !withinOperatorScope(c, target.Permissions) {
		failStaff(c, 403, "无权管理权限高于自身的账号")
		return
	}

	// 禁止修改自己的状态与权限（避免自我锁死：改自己的权限后可能再也进不来）
	if id == operatorID && (req.Status != nil || req.Permissions != nil) {
		failStaff(c, 400, "不能修改自己的启用状态或权限")
		return
	}

	if req.Nickname != nil {
		nick := strings.TrimSpace(*req.Nickname)
		if err := h.adminRepo.UpdateAdminNickname(id, nick); err != nil {
			failStaff(c, 500, "保存失败")
			return
		}
		target.Nickname = nick
	}

	if req.Permissions != nil {
		newPerms := config.NormalizePermissions(*req.Permissions)
		if !withinOperatorScope(c, newPerms) {
			failStaff(c, 403, "不能授予超出自身权限范围外的权限")
			return
		}
		if err := h.adminRepo.UpdateAdminPermissions(id, newPerms); err != nil {
			failStaff(c, 500, "保存失败")
			return
		}
		target.Permissions = newPerms
	}

	if req.Status != nil {
		st := *req.Status
		if st != 0 && st != 1 {
			failStaff(c, 400, "不支持的状态")
			return
		}
		if err := h.adminRepo.UpdateAdminStatus(id, st); err != nil {
			failStaff(c, 500, "保存失败")
			return
		}
		target.Status = st
		// 员工停用后其名下用户不再计提成（归因按 admin_user.status 判定，无需联动其它表）
	}

	// 昵称变更时确保销售推广码存在（首次访问销售页也会懒生成）
	if req.Nickname != nil {
		if _, err := h.promotionService.EnsureStaffAffCode(id); err != nil {
			log.Printf("同步员工销售推广码失败 [admin=%d]: %v", id, err)
		}
	}

	logStaffOperation(c, "admin_update", "admin_user", id,
		fmt.Sprintf("nickname=%s status=%d permissions=%s", target.Nickname, target.Status, target.Permissions))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": toAdminView(target)})
}

// ResetPassword 重置指定账号密码
// PUT /admin/admins/:id/password
func (h *AdminStaffHandler) ResetPassword(c *gin.Context) {
	if !middleware.HasPermission(c, model.WritePermission(model.PermissionSysAdmins)) {
		failStaff(c, 403, "无权限重置密码")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		failStaff(c, 400, "invalid id")
		return
	}
	// 不能重置权限高于自身的账号密码（否则可登录该账号提权）
	target, err := h.adminRepo.GetAdminByID(id)
	if err != nil || target == nil {
		failStaff(c, 404, "账号不存在")
		return
	}
	if !withinOperatorScope(c, target.Permissions) {
		failStaff(c, 403, "无权管理权限高于自身的账号")
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failStaff(c, 400, "invalid request parameters")
		return
	}
	if len(req.Password) < 6 {
		failStaff(c, 400, "密码至少 6 位")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		failStaff(c, 500, "重置失败")
		return
	}
	if err := h.adminRepo.UpdateAdminPassword(id, string(hash)); err != nil {
		failStaff(c, 500, "重置失败")
		return
	}
	logStaffOperation(c, "admin_reset_password", "admin_user", id, "")
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "密码已重置"})
}
