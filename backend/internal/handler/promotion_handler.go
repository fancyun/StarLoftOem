package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/middleware"
	"oemrpa/internal/model"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/site"
)

// PromotionHandler 推广分佣：控制台用户侧（推广码/概览/推广用户/提成收益/提现）与平台后台（提现审核/提成记录/销售业绩）
type PromotionHandler struct {
	promotionService *service.PromotionService
	finance          *service.PromotionFinanceService
	balance          *service.BalanceService
	rt               *runtime.Runtime
}

func NewPromotionHandler(promotionService *service.PromotionService, finance *service.PromotionFinanceService, balance *service.BalanceService, rt *runtime.Runtime) *PromotionHandler {
	return &PromotionHandler{promotionService: promotionService, finance: finance, balance: balance, rt: rt}
}

// failPromotion 统一错误响应（与平台既有约定一致：HTTP 200 + 业务 code）
func failPromotion(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "message": message})
}

// promotionUserID 取当前登录用户 ID（控制台推广相关接口的推广方）
func promotionUserID(c *gin.Context) (int64, bool) {
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		failPromotion(c, 401, "未登录")
		return 0, false
	}
	return userID, true
}

// Me 我的推广概览（打开推广页即开通：无推广码则自动生成并返回）
// GET /console/promotions/me
func (h *PromotionHandler) Me(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	profile, err := h.promotionService.Profile(userID)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": profile})
}

// SubUsers 下级用户列表
// GET /console/promotions/users
func (h *PromotionHandler) SubUsers(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	page, pageSize := paginationParams(c)

	users, total, err := h.promotionService.ListSubUsers(model.RefTypeReferrerUser, userID, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"list": users, "total": total, "page": page, "page_size": pageSize},
	})
}

// AdminUserPrices 账户实名两档的用户定向定价（GET /admin/users/:id/prices）
// 人脸核验/短信的定向定价已随存储拆到各产品库，入口见下方 /admin/product-config/user-prices。
func (h *PromotionHandler) AdminUserPrices(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		failPromotion(c, 400, "invalid id")
		return
	}
	items, err := h.promotionService.UnitPriceOverview(id, model.KycPricingServices)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"services": items}})
}

// AdminSetUserPrices 保存账户实名两档的用户定向定价（PUT /admin/users/:id/prices）
func (h *PromotionHandler) AdminSetUserPrices(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		failPromotion(c, 400, "invalid id")
		return
	}
	var req struct {
		Items   []*model.PriceOverride `json:"items"`
		Deleted []*model.PriceOverride `json:"deleted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "invalid request parameters")
		return
	}
	if err := h.promotionService.SaveUnitPriceOverrides(id, model.KycPricingServices, req.Items, req.Deleted); err != nil {
		failPromotion(c, 400, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// productPricingScope 校验产品作用域并返回该产品的可定价服务标识与权限
// （权限守卫只按路由前缀判定到「产品配置」这一层，具体产品在此二次判定）
func productPricingScope(c *gin.Context, product string, write bool) ([]string, bool) {
	services := model.PricingServicesByScope(product)
	if len(services) == 0 {
		failPromotion(c, 400, "不支持的产品")
		return nil, false
	}
	if !middleware.CanAccessProductConfig(c, product, write) {
		failPromotion(c, 403, "无权限访问该产品配置")
		return nil, false
	}
	return services, true
}

// AdminProductPriceUsers 定向定价选人：按手机号/用户名检索用户
// GET /admin/product-config/users?product=&keyword=
func (h *PromotionHandler) AdminProductPriceUsers(c *gin.Context) {
	if _, ok := productPricingScope(c, strings.TrimSpace(c.Query("product")), false); !ok {
		return
	}
	users, err := h.promotionService.SearchUsers(strings.TrimSpace(c.Query("keyword")), 20)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": users}})
}

// AdminProductUserPrices 产品维度的用户定向定价视图（平台价 / 当前生效价 / 自定义价）
// GET /admin/product-config/user-prices?product=&user_id=
func (h *PromotionHandler) AdminProductUserPrices(c *gin.Context) {
	product := strings.TrimSpace(c.Query("product"))
	services, ok := productPricingScope(c, product, false)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		failPromotion(c, 400, "invalid user id")
		return
	}
	items, err := h.promotionService.UnitPriceOverview(userID, services)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"product": product, "user_id": userID, "services": items},
	})
}

// AdminSetProductUserPrices 保存产品维度的用户定向定价（items 批量写入，deleted 逐项删除回落平台价）
// PUT /admin/product-config/user-prices
func (h *PromotionHandler) AdminSetProductUserPrices(c *gin.Context) {
	var req struct {
		Product string                 `json:"product" binding:"required"`
		UserID  int64                  `json:"user_id" binding:"required"`
		Items   []*model.PriceOverride `json:"items"`
		Deleted []*model.PriceOverride `json:"deleted"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "invalid request parameters")
		return
	}
	services, ok := productPricingScope(c, req.Product, true)
	if !ok {
		return
	}
	if err := h.promotionService.SaveUnitPriceOverrides(req.UserID, services, req.Items, req.Deleted); err != nil {
		failPromotion(c, 400, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// paginationParams 解析分页参数（默认第 1 页、每页 20 条，最大 200 条）
func paginationParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	return page, pageSize
}

// ============ 推广收益（控制台） ============

// Finance 我的推广收益（累计提成 / 已提现 / 可提现）
// GET /console/promotions/finance
func (h *PromotionHandler) Finance(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	view, err := h.finance.CommissionView(model.RefTypeReferrerUser, userID)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": view})
}

// FinanceLogs 我的提成流水分页
// GET /console/promotions/finance-logs
func (h *PromotionHandler) FinanceLogs(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	page, pageSize := paginationParams(c)
	list, total, err := h.promotionService.ListSettlements(model.RefTypeReferrerUser, userID, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"list": list, "total": total, "page": page, "page_size": pageSize},
	})
}

// WithdrawApply 发起提现申请（当前支持「提现到余额」：即时到账、免手续费）
// POST /console/promotions/withdraw
func (h *PromotionHandler) WithdrawApply(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	var req struct {
		Amount    float64 `json:"amount"`
		Channel   string  `json:"channel"`    // 提现方式：balance-提现到余额（当前唯一可选）
		PayeeInfo string  `json:"payee_info"` // 收款信息：线下渠道预留，余额提现无需
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "请求参数错误")
		return
	}
	w, err := h.finance.ApplyWithdraw(userID, req.Amount, req.Channel, req.PayeeInfo)
	if err != nil {
		failPromotion(c, 400, err.Error())
		return
	}
	// 站内划转（提现到余额）即时到账；线下渠道走人工审核，回「已提交」
	message := "提现成功，收益已转入平台余额"
	if w.Status != model.PromotionWithdrawPaid {
		message = "提现申请已提交，请等待平台审核"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": message, "data": w})
}

// Withdraws 我的提现申请记录
// GET /console/promotions/withdraws
func (h *PromotionHandler) Withdraws(c *gin.Context) {
	userID, ok := promotionUserID(c)
	if !ok {
		return
	}
	page, pageSize := paginationParams(c)
	list, total, err := h.finance.ListWithdraws(model.RefTypeReferrerUser, userID, -1, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"list": list, "total": total, "page": page, "page_size": pageSize},
	})
}

// ============ 推广提现审核（平台后台） ============

// AdminWithdraws 提现申请列表（referrer_type/referrer_id 不传表示全部；status 不传表示全部）
// GET /admin/promotion-withdraws
func (h *PromotionHandler) AdminWithdraws(c *gin.Context) {
	page, pageSize := paginationParams(c)
	referrerType := strings.TrimSpace(c.Query("referrer_type"))
	referrerID, _ := strconv.ParseInt(c.Query("referrer_id"), 10, 64)
	status := -1
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			status = n
		}
	}
	list, total, err := h.finance.ListWithdraws(referrerType, referrerID, status, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"list": list, "total": total, "page": page, "page_size": pageSize},
	})
}

// AdminReviewWithdraw 审核提现申请（驳回则退回可用余额）
// PUT /admin/promotion-withdraws/:id/review
func (h *PromotionHandler) AdminReviewWithdraw(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Approve      bool   `json:"approve"`
		RejectReason string `json:"reject_reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "请求参数错误")
		return
	}
	if err := h.finance.ReviewWithdraw(id, req.Approve, req.RejectReason, c.GetInt64("admin_id")); err != nil {
		failPromotion(c, 400, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// AdminMarkWithdrawPaid 标记提现已线下打款
// PUT /admin/promotion-withdraws/:id/paid
func (h *PromotionHandler) AdminMarkWithdrawPaid(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.finance.MarkWithdrawPaid(id, c.GetInt64("admin_id")); err != nil {
		failPromotion(c, 400, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// pathID 解析路径参数 :id
func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		failPromotion(c, 400, "ID 无效")
		return 0, false
	}
	return id, true
}

// ============ 提成记录（平台后台） ============

// AdminCommissions 全部提成记录（用户型推广与员工销售两表合并）
// GET /admin/promotions/commissions?referrer_type=&biz_type=&keyword=&start_date=&end_date=
func (h *PromotionHandler) AdminCommissions(c *gin.Context) {
	page, pageSize := paginationParams(c)
	list, total, err := h.promotionService.ListAllCommissions(
		strings.TrimSpace(c.Query("referrer_type")),
		strings.TrimSpace(c.Query("biz_type")),
		strings.TrimSpace(c.Query("keyword")),
		strings.TrimSpace(c.Query("start_date")),
		strings.TrimSpace(c.Query("end_date")),
		page, pageSize,
	)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"list": list, "total": total, "page": page, "page_size": pageSize},
	})
}

// ============ 员工销售业绩（平台后台） ============

// resolveSalesAdminID 解析要查看的销售员工 ID：默认当前登录管理员本人；
// 非超管仅能查看自己，超管可用 ?staff_id= 查看指定员工。
func (h *PromotionHandler) resolveSalesAdminID(c *gin.Context) (int64, bool) {
	adminID := c.GetInt64("admin_id")
	if v := strings.TrimSpace(c.Query("staff_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			failPromotion(c, 400, "invalid staff_id")
			return 0, false
		}
		if id != adminID && !middleware.IsSuperAdmin(c) {
			failPromotion(c, 403, "无权查看其他员工的销售业绩")
			return 0, false
		}
		return id, true
	}
	return adminID, true
}

// promoLink 生成推广链接（控制台注册地址 + ?ref=推广码），一律使用平台域名
func (h *PromotionHandler) promoLink(code string) string {
	if strings.TrimSpace(code) == "" {
		return ""
	}
	return site.Platform().ConsoleBase() + "/register?ref=" + code
}

// parseMonthRange 解析 ?month=YYYY-MM 为自然月区间 [startDate, endDate)：
// 返回值为 "YYYY-MM-DD" 日期串（左闭右开，endDate 为次月 1 日），供 created_at 过滤；
// 缺省或非法时取当前月。容器与数据库时区均为 Asia/Shanghai，边界一致。
func parseMonthRange(month string) (startDate, endDate, label string) {
	now := time.Now()
	year, mon := now.Year(), now.Month()
	if s := strings.TrimSpace(month); s != "" {
		if t, err := time.ParseInLocation("2006-01", s, time.Local); err == nil {
			year, mon = t.Year(), t.Month()
		}
	}
	start := time.Date(year, mon, 1, 0, 0, 0, 0, time.Local)
	return start.Format("2006-01-02"), start.AddDate(0, 1, 0).Format("2006-01-02"), start.Format("2006-01")
}

// AdminSalesSummary 销售月度报告概览（推广码/推广链接/提成比例/本月与累计提成/本月新增客户）
// GET /admin/sales/summary?staff_id=&month=YYYY-MM
func (h *PromotionHandler) AdminSalesSummary(c *gin.Context) {
	staffID, ok := h.resolveSalesAdminID(c)
	if !ok {
		return
	}
	code, err := h.promotionService.EnsureStaffAffCode(staffID)
	if err != nil {
		failPromotion(c, 500, "创建销售推广码失败")
		return
	}
	startDate, endDate, month := parseMonthRange(c.Query("month"))

	// 员工销售提成长期有效且不参与提现：累计与本月均为 staff_commission 流水合计
	totalCommission, err := h.finance.StaffCommissionInRange(model.RefTypeReferrerStaff, staffID, "", "")
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	monthCommission, err := h.finance.StaffCommissionInRange(model.RefTypeReferrerStaff, staffID, startDate, endDate)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	_, subCount, err := h.promotionService.ListSubUsers(model.RefTypeReferrerStaff, staffID, 1, 1)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	_, monthSubCount, err := h.promotionService.ListSubUsersByRange(model.RefTypeReferrerStaff, staffID, startDate, endDate, 1, 1)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"staff_id":             staffID,
			"aff_code":             code,
			"promo_link":           h.promoLink(code),
			"rate_fv":              h.promotionService.ProductRate(model.CommissionProductFV),
			"rate_sms":             h.promotionService.ProductRate(model.CommissionProductSMS),
			"month":                month,
			"month_commission":     monthCommission,
			"total_commission":     totalCommission,
			"month_sub_user_count": monthSubCount,
			"sub_user_count":       subCount,
		},
	})
}

// AdminSalesCommissions 员工销售提成流水分页（按月份过滤）
// GET /admin/sales/commissions?staff_id=&month=YYYY-MM
func (h *PromotionHandler) AdminSalesCommissions(c *gin.Context) {
	staffID, ok := h.resolveSalesAdminID(c)
	if !ok {
		return
	}
	startDate, endDate, _ := parseMonthRange(c.Query("month"))
	page, pageSize := paginationParams(c)
	list, total, err := h.promotionService.ListStaffCommissionsByRange(model.RefTypeReferrerStaff, staffID, startDate, endDate, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"list": list, "total": total, "page": page, "page_size": pageSize},
	})
}

// AdminSalesUsers 销售名下推广客户分页（按注册月份过滤）
// GET /admin/sales/users?staff_id=&month=YYYY-MM
func (h *PromotionHandler) AdminSalesUsers(c *gin.Context) {
	staffID, ok := h.resolveSalesAdminID(c)
	if !ok {
		return
	}
	startDate, endDate, _ := parseMonthRange(c.Query("month"))
	page, pageSize := paginationParams(c)
	users, total, err := h.promotionService.ListSubUsersByRange(model.RefTypeReferrerStaff, staffID, startDate, endDate, page, pageSize)
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success",
		"data": gin.H{"list": users, "total": total, "page": page, "page_size": pageSize},
	})
}
