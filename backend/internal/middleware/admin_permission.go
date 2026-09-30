package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/config"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/utils"
)

// adminRouteRule 后台路由权限规则：命中 prefix 的请求需持有 codes 中任一权限
type adminRouteRule struct {
	prefix string
	codes  []string
}

// adminRouteRules 后台路由 → 所需权限。按最长前缀匹配（如 /admin/records/recent 优先于 /admin/records）。
// 未命中任何规则的 /admin/* 路由一律拒绝（fail-closed，漏映射表现为功能不可用而非静默放行）。
var adminRouteRules = []adminRouteRule{
	// 跨产品：近期认证记录（数据统计/人脸核验页均会调用）
	{"/admin/records/recent", []string{model.PermissionSysDashboard, model.PermissionFvStats, model.PermissionFvRecords}},
	// 产品配置：接口按产品共用，handler 内再按 product 二次判定
	{"/admin/product-config", []string{model.PermissionFvProductConfig, model.PermissionSmsProductConfig}},
	// 推广提现审核
	{"/admin/promotion-withdraws", []string{model.PermissionSysAff}},
	// 短信服务
	{"/admin/sms/stats", []string{model.PermissionSmsStats}},
	{"/admin/sms/signs", []string{model.PermissionSmsSigns}},
	{"/admin/sms/templates", []string{model.PermissionSmsTemplates}},
	{"/admin/sms/records", []string{model.PermissionSmsRecords}},
	{"/admin/sms/replies", []string{model.PermissionSmsReplies}},
	{"/admin/sms/packs", []string{model.PermissionSmsPacks}},
	// 平台管理
	{"/admin/users", []string{model.PermissionSysUsers}},
	{"/admin/kyc", []string{model.PermissionSysUsers}},
	{"/admin/kyb", []string{model.PermissionSysUsers}},
	{"/admin/promotions", []string{model.PermissionSysAff}},
	{"/admin/settings", []string{model.PermissionSysSettings}},
	{"/admin/admins", []string{model.PermissionSysAdmins}},
	{"/admin/sales", []string{model.PermissionSysSales}},
	{"/admin/dashboard", []string{model.PermissionSysDashboard}},
	{"/admin/stats", []string{model.PermissionSysDashboard}},
	{"/admin/finance", []string{model.PermissionSysFinance}},
	{"/admin/bills", []string{model.PermissionSysFinance}},
	{"/admin/payments", []string{model.PermissionSysFinance}},
	// 人脸核验
	{"/admin/records", []string{model.PermissionFvRecords}},
	{"/admin/packs", []string{model.PermissionFvPacks}},
	// 运维审计（独立分组 ops）
	{"/admin/login-logs", []string{model.PermissionOpsLogs}},
	{"/admin/logs", []string{model.PermissionOpsLogs}},
	{"/admin/system", []string{model.PermissionOpsMonitor}},
	{"/admin/notify-records", []string{model.PermissionOpsNotify}},
	// 用户已购资源包：可经运维审计权限查看，或由对应产品的资源包管理权限查看（页面已归入 fv/sms 分区）
	{"/admin/user-packs", []string{model.PermissionOpsUserPacks, model.PermissionFvPacks, model.PermissionSmsPacks}},
	{"/admin/api-keys", []string{model.PermissionOpsAPIKeys}},
	{"/admin/uploads", []string{model.PermissionOpsUploads}},
	{"/admin/promoters", []string{model.PermissionOpsPromoters}},
}

// adminPermissionWhitelist 登录态即可访问、无需额外权限的后台路由（本人信息与改密）
var adminPermissionWhitelist = map[string]bool{
	"/admin/me":              true,
	"/admin/change-password": true,
}

// RequiredPermissions 返回指定请求所需的权限码集合；第二个返回值为是否存在匹配规则。
// 读/写判定：GET（含 HEAD/OPTIONS）只要求模块读权限；其余方法要求对应写权限 `<模块>.write`
// （写隐含读，故无需再叠加读码）。
func RequiredPermissions(method, fullPath string) ([]string, bool) {
	var best *adminRouteRule
	for i := range adminRouteRules {
		r := &adminRouteRules[i]
		if strings.HasPrefix(fullPath, r.prefix) && (best == nil || len(r.prefix) > len(best.prefix)) {
			best = r
		}
	}
	if best == nil {
		return nil, false
	}
	if isReadMethod(method) {
		return best.codes, true
	}
	out := make([]string, 0, len(best.codes))
	for _, code := range best.codes {
		out = append(out, model.WritePermission(code))
	}
	return out, true
}

// isReadMethod 只读方法；其余方法一律按写权限判定
func isReadMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// AdminPermissions 取当前登录管理员的权限串（由 AdminPermissionGuard 注入）
func AdminPermissions(c *gin.Context) string {
	v, _ := c.Get("admin_permissions")
	s, _ := v.(string)
	return s
}

// HasPermission 当前登录管理员是否持有 codes 中任一权限（超级管理员恒为真）
func HasPermission(c *gin.Context, codes ...string) bool {
	if IsSuperAdmin(c) {
		return true
	}
	perms := AdminPermissions(c)
	for _, code := range codes {
		if config.HasPermission(perms, code) {
			return true
		}
	}
	return false
}

// IsSuperAdmin 是否超级管理员：仅取决于是否持有 all 权限（与账号 id 无关）
func IsSuperAdmin(c *gin.Context) bool {
	return config.HasPermission(AdminPermissions(c), model.PermissionAll)
}

// CanAccessProductConfig 当前账号对指定产品（fv / sms）的配置是否具备读（write=false）/写（write=true）权限。
// 权限守卫只按路由前缀判定到「产品配置」这一层，具体产品由调用方（产品配置、用户定向定价等接口）二次判定。
func CanAccessProductConfig(c *gin.Context, product string, write bool) bool {
	var code string
	switch product {
	case model.ProductFV:
		code = model.PermissionFvProductConfig
	case model.ServiceSMS:
		code = model.PermissionSmsProductConfig
	default:
		return false
	}
	if write {
		code = model.WritePermission(code)
	}
	return HasPermission(c, code)
}

// AdminPermissionGuard 后台权限守卫（组级中间件）：
// 校验账号在用状态 → 注入 admin_id/权限上下文 → 按路由前缀判定所需权限。
// 停用账号的旧 token 在此被拦下（不必等 token 过期）。
func AdminPermissionGuard(adminRepo *repository.AdminRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminID := c.GetInt64("user_id")
		admin, err := adminRepo.GetAdminByID(adminID)
		if err != nil || admin == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号不存在"})
			c.Abort()
			return
		}
		if admin.Status != 1 {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "账号已停用"})
			c.Abort()
			return
		}
		c.Set("admin_id", admin.ID)
		c.Set("admin_username", admin.Username)
		c.Set("admin_nickname", admin.Nickname)
		c.Set("admin_permissions", admin.Permissions)

		fullPath := c.FullPath()
		if adminPermissionWhitelist[fullPath] {
			c.Next()
			return
		}

		codes, ok := RequiredPermissions(c.Request.Method, fullPath)
		if !ok {
			utils.AdminLogger.Printf("admin_id=%d 权限规则未覆盖后台路由：%s %s", admin.ID, c.Request.Method, fullPath)
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权限访问该功能"})
			c.Abort()
			return
		}
		if !HasPermission(c, codes...) {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权限访问该功能"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CheckAdminRouteCoverage 启动期自检：对未配置权限规则的后台路由打告警日志（不阻断启动）。
// 目的：新增后台路由却忘记登记权限时能及早发现，避免该路由直接 403 而无人知晓。
func CheckAdminRouteCoverage(routes []gin.RouteInfo) {
	for _, rt := range routes {
		if !strings.HasPrefix(rt.Path, "/admin") {
			continue
		}
		if rt.Path == "/admin/login" || adminPermissionWhitelist[rt.Path] {
			continue
		}
		if _, ok := RequiredPermissions(rt.Method, rt.Path); !ok {
			utils.AdminLogger.Printf("权限规则未覆盖后台路由：%s %s", rt.Method, rt.Path)
		}
	}
}
