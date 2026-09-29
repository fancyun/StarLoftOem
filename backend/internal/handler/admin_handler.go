package handler

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"oemrpa/internal/config"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var adminValidator = utils.NewInputValidator()

type AdminHandler struct {
	adminRepo           *repository.AdminRepository
	userRepo            *repository.UserRepository
	authRecordRepo      *repository.AuthRecordRepository
	paymentRepo         *repository.PaymentOrderRepository
	billRepo            *repository.BillRepository
	resourcePackRepo    *repository.ResourcePackRepository
	smsResourcePackRepo *repository.SmsResourcePackRepository
	loginLogRepo        *repository.LoginLogRepository
	balanceSvc          *service.BalanceService
	authSvc             *service.AuthService
	rt                  *runtime.Runtime
	jwtSecret           string
}

func NewAdminHandler(
	adminRepo *repository.AdminRepository,
	userRepo *repository.UserRepository,
	authRecordRepo *repository.AuthRecordRepository,
	paymentRepo *repository.PaymentOrderRepository,
	billRepo *repository.BillRepository,
	resourcePackRepo *repository.ResourcePackRepository,
	smsResourcePackRepo *repository.SmsResourcePackRepository,
	loginLogRepo *repository.LoginLogRepository,
	balanceSvc *service.BalanceService,
	authSvc *service.AuthService,
	rt *runtime.Runtime,
	jwtSecret string,
) *AdminHandler {
	return &AdminHandler{
		adminRepo:           adminRepo,
		userRepo:            userRepo,
		authRecordRepo:      authRecordRepo,
		paymentRepo:         paymentRepo,
		billRepo:            billRepo,
		resourcePackRepo:    resourcePackRepo,
		smsResourcePackRepo: smsResourcePackRepo,
		loginLogRepo:        loginLogRepo,
		balanceSvc:          balanceSvc,
		authSvc:             authSvc,
		rt:                  rt,
		jwtSecret:           jwtSecret,
	}
}

// logAdminOperation 记录管理员操作日志（仅写入 admin.log 文件，不额外建表）
func (h *AdminHandler) logAdminOperation(c *gin.Context, operation, resourceType string, resourceID int64, details string) {
	adminID, _ := c.Get("user_id")
	adminIDInt, _ := adminID.(int64)

	// 写入文件日志
	utils.AdminLogger.Printf("admin_id=%d operation=%s resource_type=%s resource_id=%d ip=%s details=%s",
		adminIDInt, operation, resourceType, resourceID, c.ClientIP(), details)
}

// ---------- 认证记录聚合（人脸核验 FV 库） ----------
// 认证记录仅存储于人脸核验库 oem_fv。

// recordRepos 返回认证记录仓库（FV）
func (h *AdminHandler) recordRepos() []*repository.AuthRecordRepository {
	return []*repository.AuthRecordRepository{h.authRecordRepo}
}

// aggregateDailyRecordStats 两库每日记录数按天求和
func (h *AdminHandler) aggregateDailyRecordStats(startDate, endDate string) (map[string]int64, error) {
	result := make(map[string]int64)
	for _, repo := range h.recordRepos() {
		m, err := repo.GetDailyRecordStats(startDate, endDate)
		if err != nil {
			return nil, err
		}
		for d, c := range m {
			result[d] += c
		}
	}
	return result, nil
}

// getDailyRecordStatsAggregated 聚合每日记录数（对外统一入口）
func (h *AdminHandler) getDailyRecordStatsAggregated(startDate, endDate string) (map[string]int64, error) {
	return h.aggregateDailyRecordStats(startDate, endDate)
}

// aggregateDailyIncomeStats 两库每日收入按天相加
func (h *AdminHandler) aggregateDailyIncomeStats(startDate, endDate string) (map[string]float64, error) {
	result := make(map[string]float64)
	for _, repo := range h.recordRepos() {
		m, err := repo.GetDailyIncomeStats(startDate, endDate)
		if err != nil {
			return nil, err
		}
		for d, amount := range m {
			result[d] += amount
		}
	}
	return result, nil
}

// getDailyIncomeStatsAggregated 聚合每日收入（对外统一入口）
func (h *AdminHandler) getDailyIncomeStatsAggregated(startDate, endDate string) (map[string]float64, error) {
	return h.aggregateDailyIncomeStats(startDate, endDate)
}

// getAllRecordsAggregated 两库记录列表各取一页后按创建时间倒序合并再截取，总数求和
func (h *AdminHandler) getAllRecordsAggregated(page, pageSize int, status *int, userID *int64) ([]*model.AuthRecord, int64, error) {
	var total int64
	all := make([]*model.AuthRecord, 0)
	for _, repo := range h.recordRepos() {
		records, t, err := repo.GetAllRecords(page, pageSize, status, userID)
		if err != nil {
			return nil, 0, err
		}
		total += t
		all = append(all, records...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	offset := (page - 1) * pageSize
	if offset >= len(all) {
		return []*model.AuthRecord{}, total, nil
	}
	end := offset + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// getUserAuthRecordsAggregated 两库用户记录各取一页后合并排序截取，总数求和
func (h *AdminHandler) getUserAuthRecordsAggregated(userID int64, page, pageSize int) ([]*model.AuthRecord, int64, error) {
	var total int64
	all := make([]*model.AuthRecord, 0)
	for _, repo := range h.recordRepos() {
		records, t, err := repo.GetUserAuthRecords(userID, page, pageSize)
		if err != nil {
			return nil, 0, err
		}
		total += t
		all = append(all, records...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	offset := (page - 1) * pageSize
	if offset >= len(all) {
		return []*model.AuthRecord{}, total, nil
	}
	end := offset + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

// getRecentRecordsAggregated 两库最近记录按创建时间合并，取前 limit 条
func (h *AdminHandler) getRecentRecordsAggregated(limit int) ([]*repository.RecentAuthRecord, error) {
	recent := make([]*repository.RecentAuthRecord, 0)
	for _, repo := range h.recordRepos() {
		records, err := repo.GetRecentRecords(limit)
		if err != nil {
			return nil, err
		}
		recent = append(recent, records...)
	}
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].CreatedAt.After(recent[j].CreatedAt)
	})
	if len(recent) > limit {
		recent = recent[:limit]
	}
	return recent, nil
}

// AdminLogin 管理员登录
func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 查询管理员
	admin, err := h.adminRepo.GetAdminByUsername(req.Username)
	if err != nil {
		log.Printf("Admin login failed: username=%s, error=%v", req.Username, err)
		h.loginLogRepo.InsertAdminLoginLog(&model.AdminLoginLog{
			Username:   req.Username,
			IP:         c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Status:     0,
			FailReason: "account not found",
		})
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "invalid username or password",
		})
		return
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Printf("Admin login failed: username=%s, password mismatch", req.Username)
		h.loginLogRepo.InsertAdminLoginLog(&model.AdminLoginLog{
			AdminID:    admin.ID,
			Username:   req.Username,
			IP:         c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Status:     0,
			FailReason: "password mismatch",
		})
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "invalid username or password",
		})
		return
	}

	// 检查账号状态
	if admin.Status != 1 {
		h.loginLogRepo.InsertAdminLoginLog(&model.AdminLoginLog{
			AdminID:    admin.ID,
			Username:   req.Username,
			IP:         c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Status:     0,
			FailReason: "account disabled",
		})
		c.JSON(http.StatusOK, gin.H{
			"code":    403,
			"message": "account disabled",
		})
		return
	}

	// 更新最后登录时间
	err = h.adminRepo.UpdateLastLoginTime(admin.ID)
	if err != nil {
		log.Printf("Failed to update admin last login time: %v", err)
	}

	// 记录管理员登录日志（成功）
	h.loginLogRepo.InsertAdminLoginLog(&model.AdminLoginLog{
		AdminID:   admin.ID,
		Username:  req.Username,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    1,
	})

	// 生成 JWT Token
	jwtManager := utils.NewJWTManager(h.jwtSecret)
	token, err := jwtManager.GenerateToken(admin.ID, admin.Username, "admin", 24*time.Hour)
	if err != nil {
		log.Printf("Failed to generate admin token: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to generate token",
		})
		return
	}

	// 记录管理员登录操作日志（登录发生在鉴权之前，需手动设置 admin_id）
	c.Set("user_id", admin.ID)
	h.logAdminOperation(c, "login", "admin", admin.ID, admin.Username)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"admin_id":    admin.ID,
			"username":    admin.Username,
			"nickname":    admin.Nickname,
			"permissions": config.ParsePermissions(admin.Permissions),
			"is_super":    config.HasPermission(admin.Permissions, model.PermissionAll),
			"token":       token,
			"expire_in":   86400,
		},
	})
}

// GetStatisticsOverview 获取统计概览
func (h *AdminHandler) GetStatisticsOverview(c *gin.Context) {
	// 统计用户数
	_, totalUsers, err := h.userRepo.GetAllUsers(1, 1, "", nil, "", "")
	if err != nil {
		totalUsers = 0
	}

	today := time.Now().Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	// 今日订单数（两库聚合）
	todayRecordStats, _ := h.getDailyRecordStatsAggregated(today, today)
	var todayRecords int64 = 0
	if v, ok := todayRecordStats[today]; ok {
		todayRecords = v
	}

	// 今日认证收入（仅统计已完成认证记录，两库聚合；FV 数据统计页使用）
	todayIncomeStats, _ := h.getDailyIncomeStatsAggregated(today, today)
	var todayRevenue float64 = 0
	if v, ok := todayIncomeStats[today]; ok {
		todayRevenue = v
	}

	// 本月认证收入（仅统计已完成认证记录，两库聚合）
	monthIncomeStats, _ := h.getDailyIncomeStatsAggregated(monthStart, today)
	var monthRevenue float64 = 0
	for _, v := range monthIncomeStats {
		monthRevenue += v
	}

	// 今日 / 本月支付收入（支付记录口径：已支付支付单按支付时间统计，含支付宝/微信/人工支付）
	var todayPaymentAmount, monthPaymentAmount float64
	if v, err := h.paymentRepo.SumPaidAmount("DATE(paid_at) = ?", today); err == nil {
		todayPaymentAmount = v
	} else {
		log.Printf("Failed to sum today payment amount: %v", err)
	}
	if v, err := h.paymentRepo.SumPaidAmount("DATE(paid_at) >= ?", monthStart); err == nil {
		monthPaymentAmount = v
	} else {
		log.Printf("Failed to sum month payment amount: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total_users":          totalUsers,
			"today_orders":         todayRecords,
			"today_revenue":        todayRevenue,
			"month_revenue":        monthRevenue,
			"today_payment_amount": todayPaymentAmount,
			"month_payment_amount": monthPaymentAmount,
		},
	})
}

// GetOrderStatistics 获取订单统计（按日期）
func (h *AdminHandler) GetOrderStatistics(c *gin.Context) {
	days := c.DefaultQuery("days", "7")
	dayNum, err := strconv.Atoi(days)
	if err != nil || dayNum <= 0 || dayNum > 90 {
		dayNum = 7
	}

	startDate := time.Now().AddDate(0, 0, -(dayNum - 1)).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	stats, err := h.getDailyRecordStatsAggregated(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get order statistics",
		})
		return
	}

	dates := make([]string, 0, dayNum)
	counts := make([]int64, 0, dayNum)
	for i := dayNum - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, d)
		counts = append(counts, stats[d])
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"dates":  dates,
			"counts": counts,
		},
	})
}

// GetIncomeStatistics 获取收入统计（按日期）
// source=payment 时按支付记录统计（已支付支付单按支付时间归日，平台首页收入趋势使用）；
// 默认按认证记录统计（已完成认证记录，FV 数据统计页使用）。
func (h *AdminHandler) GetIncomeStatistics(c *gin.Context) {
	days := c.DefaultQuery("days", "7")
	dayNum, err := strconv.Atoi(days)
	if err != nil || dayNum <= 0 || dayNum > 90 {
		dayNum = 7
	}

	startDate := time.Now().AddDate(0, 0, -(dayNum - 1)).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	var stats map[string]float64
	if c.Query("source") == "payment" {
		stats, err = h.paymentRepo.GetDailyPaidAmounts(startDate, endDate)
	} else {
		stats, err = h.getDailyIncomeStatsAggregated(startDate, endDate)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get income statistics",
		})
		return
	}

	dates := make([]string, 0, dayNum)
	amounts := make([]float64, 0, dayNum)
	for i := dayNum - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, d)
		amounts = append(amounts, stats[d])
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"dates":   dates,
			"amounts": amounts,
		},
	})
}

// GetRecentAuthRecords 获取最近认证记录列表
func (h *AdminHandler) GetRecentAuthRecords(c *gin.Context) {
	limit := 10
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	records, err := h.getRecentRecordsAggregated(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get recent records",
		})
		return
	}

	list := make([]gin.H, 0, len(records))
	for _, o := range records {
		list = append(list, gin.H{
			"biz_no":     o.BizNo,
			"user_phone": o.UserPhone,
			"name":       o.Name,
			"status":     o.Status,
			"cost":       o.Cost,
			"created_at": o.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list": list,
		},
	})
}

// ManualRegisterUser 管理员手动注册用户
func (h *AdminHandler) ManualRegisterUser(c *gin.Context) {
	var req struct {
		Phone    string `json:"phone" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 验证手机号格式
	if err := adminValidator.ValidatePhone(req.Phone); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid phone format",
		})
		return
	}

	// 校验用户名格式（仅支持英文+数字+下划线）
	req.Username = strings.TrimSpace(req.Username)
	if !service.ValidateUsername(req.Username) {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "用户名仅支持英文、数字、下划线，长度3-32位",
		})
		return
	}

	// 验证密码长度
	if len(req.Password) < 6 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "password must be at least 6 characters",
		})
		return
	}

	// 检查手机号是否已存在
	exists, err := h.userRepo.CheckPhoneExists(req.Phone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to check phone existence",
		})
		return
	}
	if exists {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "phone already exists",
		})
		return
	}

	// 检查用户名是否已存在
	exists, err = h.userRepo.CheckUsernameExists(req.Username)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to check username existence",
		})
		return
	}
	if exists {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "username already exists",
		})
		return
	}

	// 密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to hash password",
		})
		return
	}

	// 创建用户
	user := &model.User{
		Phone:          req.Phone,
		Username:       req.Username,
		PasswordHash:   string(hashedPassword),
		Balance:        0,
		RealnameStatus: model.RealnameNone,
		Status:         1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = h.userRepo.CreateUser(user)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to create user",
		})
		return
	}

	adminID := c.GetInt64("user_id")
	log.Printf("Admin %d manually registered user: phone=%s", adminID, req.Phone)

	// 记录操作日志
	h.logAdminOperation(c, "register", "user", user.ID, fmt.Sprintf("phone=%s username=%s", req.Phone, req.Username))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "user registered successfully",
		"data": gin.H{
			"id":       user.ID,
			"phone":    user.Phone,
			"username": user.Username,
		},
	})
}

// GetAuthRecordList 获取认证记录列表
func (h *AdminHandler) GetAuthRecordList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	statusStr := c.Query("status")
	userIDStr := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var status *int
	var userID *int64

	if statusStr != "" {
		s, err := strconv.Atoi(statusStr)
		if err == nil {
			status = &s
		}
	}

	if userIDStr != "" {
		uid, err := strconv.ParseInt(userIDStr, 10, 64)
		if err == nil {
			userID = &uid
		}
	}

	records, total, err := h.getAllRecordsAggregated(page, pageSize, status, userID)
	if err != nil {
		log.Printf("Failed to get auth record list: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get record list",
		})
		return
	}

	recordList := make([]gin.H, 0, len(records))
	for _, record := range records {
		recordList = append(recordList, gin.H{
			"id":             record.ID,
			"biz_no":         record.BizNo,
			"user_id":        record.UserID,
			"user_phone":     record.UserPhone,
			"status":         record.Status,
			"cost":           record.Cost,
			"pay_type":       record.PayType,
			"pack_count":     record.PackCount,
			"result_code":    record.ResultCode,
			"result_message": record.ResultMessage,
			"is_refunded":    record.IsRefunded,
			"created_at":     record.CreatedAt,
			"finished_at":    record.FinishedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      recordList,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetAuthRecordDetail 获取认证记录详情
func (h *AdminHandler) GetAuthRecordDetail(c *gin.Context) {
	recordIDStr := c.Param("id")
	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid record id",
		})
		return
	}

	// 认证记录存储于 FV 库
	record, err := h.authRecordRepo.GetRecordByID(recordID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "record not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":             record.ID,
			"biz_no":         record.BizNo,
			"user_id":        record.UserID,
			"return_url":     record.ReturnURL,
			"notify_url":     record.NotifyURL,
			"biz_extra_data": record.BizExtraData,
			"up_token":       record.UpToken,
			"up_biz_id":      record.UpBizID,
			"up_request_id":  record.UpRequestID,
			"result_code":    record.ResultCode,
			"result_message": record.ResultMessage,
			"result_data":    record.ResultData,
			"status":         record.Status,
			"cost":           record.Cost,
			"pay_type":       record.PayType,
			"pack_count":     record.PackCount,
			"is_refunded":    record.IsRefunded,
			"notify_times":   record.NotifyTimes,
			"notify_status":  record.NotifyStatus,
			"created_at":     record.CreatedAt,
			"updated_at":     record.UpdatedAt,
			"finished_at":    record.FinishedAt,
		},
	})
}

// QueryAuthRecordResult 手动查询认证记录的上游结果（调上游核对并回写本地）
// POST /admin/records/:id/query-result
func (h *AdminHandler) QueryAuthRecordResult(c *gin.Context) {
	recordID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid record id",
		})
		return
	}

	record, queryMessage, err := h.authSvc.QueryRecordResultForAdmin(recordID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	h.logAdminOperation(c, "query_result", "auth_record", recordID, queryMessage)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":             record.ID,
			"biz_no":         record.BizNo,
			"status":         record.Status,
			"result_code":    record.ResultCode,
			"result_message": record.ResultMessage,
			"is_refunded":    record.IsRefunded,
			"up_query_count": record.UpQueryCount,
			"finished_at":    record.FinishedAt,
			"query_message":  queryMessage,
		},
	})
}

// GetPaymentOrderList 获取支付订单列表
func (h *AdminHandler) GetPaymentOrderList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	statusStr := c.Query("status")
	userIDStr := c.Query("user_id")
	channelStr := c.Query("channel")
	payOrderNoStr := c.Query("pay_order_no")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var status *int
	var userID *int64

	if statusStr != "" {
		s, err := strconv.Atoi(statusStr)
		if err == nil {
			status = &s
		}
	}

	if userIDStr != "" {
		uid, err := strconv.ParseInt(userIDStr, 10, 64)
		if err == nil {
			userID = &uid
		}
	}

	var channel *string
	if channelStr != "" {
		channel = &channelStr
	}
	var payOrderNo *string
	if payOrderNoStr != "" {
		payOrderNo = &payOrderNoStr
	}

	orders, total, err := h.paymentRepo.GetAllOrders(page, pageSize, status, userID, channel, payOrderNo)
	if err != nil {
		log.Printf("Failed to get payment order list: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get order list",
		})
		return
	}

	orderList := make([]gin.H, 0, len(orders))
	for _, order := range orders {
		orderList = append(orderList, gin.H{
			"id":               order.ID,
			"pay_order_no":     order.PayOrderNo,
			"user_id":          order.UserID,
			"user_phone":       order.UserPhone,
			"amount":           order.Amount,
			"channel":          order.Channel,
			"channel_trade_no": order.ChannelTradeNo,
			"bank_serial_no":   order.BankSerialNo,
			"status":           order.Status,
			"intent":           order.Intent,
			"biz_no":           order.BizNo,
			"balance_amount":   order.BalanceAmount,
			"refund_status":    order.RefundStatus,
			"refund_amount":    order.RefundAmount,
			"paid_at":          order.PaidAt,
			"refunded_at":      order.RefundedAt,
			"created_at":       order.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      orderList,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// QueryPaymentResult 管理后台手动查询支付结果：向渠道查询待支付单真实状态并落地
// （渠道已支付→补账/发放；渠道已关闭→关闭本地单），不做人工强制入账。
// POST /admin/payments/:id/query-result
func (h *AdminHandler) QueryPaymentResult(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "支付单 ID 无效"})
		return
	}

	result, err := h.balanceSvc.QueryPaymentResult(id, h.alipay(), h.wechatPay())
	if err != nil {
		h.logAdminOperation(c, "payment_query_result", "payment_order", id, fmt.Sprintf("failed: %v", err))
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.logAdminOperation(c, "payment_query_result", "payment_order", id, fmt.Sprintf("result=%s", result))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"result": result}})
}

// alipay / wechatPay 返回当前生效的支付客户端（未配置时为 nil）
func (h *AdminHandler) alipay() *upstream.AlipayClient { return h.rt.Alipay() }

func (h *AdminHandler) wechatPay() *upstream.WechatPayClient { return h.rt.WechatPay() }

// GetUserDetail 获取用户详情
func (h *AdminHandler) GetUserDetail(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid user id",
		})
		return
	}

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "user not found",
		})
		return
	}

	// 提取 NullString 的值
	kycName := ""
	if user.VerifiedName.Valid {
		kycName = user.VerifiedName.String
	}
	kycIDCard := ""
	if user.VerifiedNumber.Valid {
		kycIDCard = user.VerifiedNumber.String
	}

	personalFreeRemaining, err := h.authSvc.GetFreeAuthRemaining(userID)
	if err != nil {
		personalFreeRemaining = -1
	}
	enterpriseFreeRemaining, err := h.authSvc.GetKybFreeAuthRemaining(userID)
	if err != nil {
		enterpriseFreeRemaining = -1
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":                        user.ID,
			"phone":                     user.Phone,
			"username":                  user.Username,
			"balance":                   user.Balance,
			"realname_status":           user.RealnameStatus,
			"verified_name":             kycName,
			"verified_number":           kycIDCard,
			"status":                    user.Status,
			"last_login_at":             user.LastLoginAt,
			"created_at":                user.CreatedAt,
			"updated_at":                user.UpdatedAt,
			"personal_free_remaining":   personalFreeRemaining,   // 个人实名剩余免费次数
			"enterprise_free_remaining": enterpriseFreeRemaining, // 企业实名剩余免费次数
		},
	})
}

// UpdateUserStatus 更新用户状态
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid user id",
		})
		return
	}

	var req struct {
		Status int `json:"status" binding:"required"` // 0-禁用 1-正常
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	if req.Status != 0 && req.Status != 1 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid status value",
		})
		return
	}

	err = h.userRepo.UpdateUserStatus(userID, req.Status)
	if err != nil {
		log.Printf("Failed to update user status: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to update user status",
		})
		return
	}

	// 记录操作日志
	statusText := "禁用"
	if req.Status == 1 {
		statusText = "启用"
	}
	h.logAdminOperation(c, "user_status", "user", userID, "status="+statusText)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ResetUserKycFree 管理员重置用户免费实名次数（POST /admin/users/:id/reset-kyc-free）
func (h *AdminHandler) ResetUserKycFree(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid user id",
		})
		return
	}

	var req struct {
		Type string `json:"type"` // personal-个人实名 enterprise-企业实名
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Type != "personal" && req.Type != "enterprise") {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid reset type",
		})
		return
	}

	personalRemaining, enterpriseRemaining, err := h.authSvc.ResetRealnameFreeBase(userID, req.Type)
	if err != nil {
		log.Printf("Failed to reset kyc free count: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to reset kyc free count",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "user_kyc_free_reset", "user", userID, "type="+req.Type)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"personal_remaining":   personalRemaining,
			"enterprise_remaining": enterpriseRemaining,
		},
	})
}

// GrantFvTestPack 管理员发放人脸核验测试资源包（POST /admin/packs/grant-test）
// product 取 fv_auth（有源）或 fv_self（无源），资源包按子产品精确匹配，两者互不通用；count <= 0 取默认 5 次。
func (h *AdminHandler) GrantFvTestPack(c *gin.Context) {
	h.grantTestPack(c, model.ServiceFVAuth, model.ServiceFVSelf)
}

// GrantSmsTestPack 管理员发放短信测试资源包（POST /admin/sms/packs/grant-test）
// product 取 sms（验证码/通知）或 sms_marketing（营销）；count <= 0 取默认 20 条。
func (h *AdminHandler) GrantSmsTestPack(c *gin.Context) {
	h.grantTestPack(c, model.ServiceSMS, model.ProductSMSMarketing)
}

// grantTestPack 向指定用户发放测试资源包（免费发放不计入资金账单）；
// product 须在 allowed 白名单内，count 为负数视为非法请求。
func (h *AdminHandler) grantTestPack(c *gin.Context, allowed ...string) {
	var req struct {
		UserID  int64  `json:"user_id" binding:"required"`
		Product string `json:"product" binding:"required"`
		Count   int    `json:"count"` // 发放数量，<=0 时取默认值
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}
	matched := false
	for _, p := range allowed {
		if req.Product == p {
			matched = true
			break
		}
	}
	if !matched || req.Count < 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	grant, err := h.balanceSvc.GrantTestPack(c.GetInt64("user_id"), req.UserID, req.Product, req.Count)
	if err != nil {
		log.Printf("Failed to grant test resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	h.logAdminOperation(c, "pack_grant_test", "user", req.UserID,
		fmt.Sprintf("product=%s count=%d user_pack_id=%d", grant.Product, grant.Count, grant.UserPackID))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "发放成功",
		"data": gin.H{
			"user_pack_id": grant.UserPackID,
			"product":      grant.Product,
			"count":        grant.Count,
			"pack_name":    grant.PackName,
		},
	})
}

// normalizeBankSerialNo 清理人工入账唯一账单键（账号_记账日期_流水号）各段前后的空白与制表符，保持段内内容不变
func normalizeBankSerialNo(v string) string {
	parts := strings.Split(strings.TrimSpace(v), "_")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return strings.Join(parts, "_")
}

// RechargeUserBalance 为用户手动充值
func (h *AdminHandler) RechargeUserBalance(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid user id",
		})
		return
	}

	var req struct {
		Amount       float64 `json:"amount" binding:"required"`
		BankSerialNo string  `json:"bank_serial_no" binding:"required"` // 银行流水单号（必填，用于对账）
		Remark       string  `json:"remark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "amount must be greater than 0",
		})
		return
	}

	// 验证金额范围
	if err := adminValidator.ValidateAmount(req.Amount); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "amount must be between 0 and 1,000,000",
		})
		return
	}

	// 校验银行流水单号（账号_记账日期_流水号 三段各自去空格，避免同单因空格差异重复入账）
	req.BankSerialNo = adminValidator.SanitizeString(normalizeBankSerialNo(req.BankSerialNo))
	if len(req.BankSerialNo) < 4 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "请填写有效的银行流水单号（至少4位）",
		})
		return
	}

	// 清理备注中的潜在XSS
	req.Remark = adminValidator.SanitizeString(req.Remark)

	adminID := c.GetInt64("user_id")
	remark := req.Remark
	if remark == "" {
		remark = fmt.Sprintf("管理员人工充值(admin_id:%d)", adminID)
	} else {
		remark = fmt.Sprintf("管理员人工充值(admin_id:%d): %s", adminID, req.Remark)
	}

	// 人工充值（type=1，增加余额）
	err = h.balanceSvc.ManualRechargeBalance(userID, req.Amount, remark, req.BankSerialNo)
	if err != nil {
		log.Printf("Failed to recharge user balance: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to recharge balance",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "recharge", "user", userID, fmt.Sprintf("amount=%.2f bank_serial_no=%s", req.Amount, req.BankSerialNo))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "充值成功",
		"data": gin.H{
			"user_id":        userID,
			"amount":         req.Amount,
			"bank_serial_no": req.BankSerialNo,
			"remark":         remark,
		},
	})
}

// GetUserList 获取用户列表
func (h *AdminHandler) GetUserList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	phone := c.Query("phone")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 实名状态筛选：0-未实名 1-个人实名 2-企业实名（空为不限）
	var realnameStatus *int
	if v := c.Query("realname_status"); v != "" {
		if s, err := strconv.Atoi(v); err == nil && s >= model.RealnameNone && s <= model.RealnameEnterprise {
			realnameStatus = &s
		}
	}

	users, total, err := h.userRepo.GetAllUsers(page, pageSize, phone, realnameStatus, startDate, endDate)
	if err != nil {
		log.Printf("Failed to get user list: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get user list",
		})
		return
	}

	// 数据脱敏处理
	userList := make([]gin.H, 0, len(users))
	for _, user := range users {
		// 提取 NullString 的值，避免序列化为 {"String":"...","Valid":true}
		kycName := ""
		if user.VerifiedName.Valid {
			kycName = user.VerifiedName.String
		}
		kycIDCard := ""
		if user.VerifiedNumber.Valid {
			kycIDCard = user.VerifiedNumber.String
		}

		userList = append(userList, gin.H{
			"id":              user.ID,
			"phone":           user.Phone,
			"username":        user.Username,
			"balance":         user.Balance,
			"realname_status": user.RealnameStatus,
			"verified_name":   kycName,
			"verified_number": kycIDCard,
			"status":          user.Status,
			"last_login_at":   user.LastLoginAt,
			"created_at":      user.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      userList,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetUserFinanceStats 获取用户财务统计（充值/消费/退款，数据源 bill）
func (h *AdminHandler) GetUserFinanceStats(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	stats, err := h.billRepo.Stats(userID)
	if err != nil {
		log.Printf("Failed to get user finance stats: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get finance stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalRecharge": stats["totalRecharge"],
			"totalConsume":  stats["totalConsume"],
			"totalRefund":   stats["totalRefund"],
		},
	})
}

// GetUserBalanceLogs 获取用户余额流水（数据源 bill，字段与旧 balance_log 对齐：type/amount/balance_after/bank_serial_no）
func (h *AdminHandler) GetUserBalanceLogs(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	bills, total, err := h.billRepo.GetUserBills(userID, page, pageSize)
	if err != nil {
		log.Printf("Failed to get user balance logs: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get balance logs",
		})
		return
	}

	logs := make([]gin.H, 0, len(bills))
	for _, b := range bills {
		logs = append(logs, gin.H{
			"id":             b.ID,
			"type":           b.BillType,
			"amount":         b.Amount,
			"balance_after":  b.BalanceAfter,
			"bank_serial_no": b.BankSerialNo,
			"remark":         b.Remark,
			"created_at":     b.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetBills 获取统一账单列表（可按用户/产品/账单类型/花费类型过滤）
func (h *AdminHandler) GetBills(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.DefaultQuery("user_id", "0"), 10, 64)
	billType, _ := strconv.Atoi(c.DefaultQuery("bill_type", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	bills, total, err := h.billRepo.List(&repository.BillFilter{
		UserID:    userID,
		Product:   c.DefaultQuery("product", ""),
		BillType:  billType,
		SpendType: c.DefaultQuery("spend_type", ""),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		log.Printf("Failed to get bills: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get bills",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      bills,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetUserAuthRecords 获取用户认证记录
func (h *AdminHandler) GetUserAuthRecords(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	records, total, err := h.getUserAuthRecordsAggregated(userID, page, pageSize)
	if err != nil {
		log.Printf("Failed to get user auth records: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get auth records",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      records,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// ChangePassword 修改管理员密码
func (h *AdminHandler) ChangePassword(c *gin.Context) {
	adminIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "unauthorized",
		})
		return
	}
	adminID, ok := adminIDVal.(int64)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": "invalid admin identity",
		})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 查询当前管理员
	admin, err := h.adminRepo.GetAdminByID(adminID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "admin not found",
		})
		return
	}

	// 校验旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid old password",
		})
		return
	}

	// 校验新密码（与创建管理员一致，至少6位）
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "password must be at least 6 characters",
		})
		return
	}

	// 生成新密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash new password: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to hash password",
		})
		return
	}

	// 更新密码
	if err := h.adminRepo.UpdateAdminPassword(adminID, string(hashedPassword)); err != nil {
		log.Printf("Failed to update admin password: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to change password",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "change_password", "admin", adminID, "")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ---------- 资源包管理（管理后台） ----------

// GetResourcePackList 获取资源包列表（管理后台，含已下架）
func (h *AdminHandler) GetResourcePackList(c *gin.Context) {
	packs, err := h.resourcePackRepo.ListPacks(nil)
	if err != nil {
		log.Printf("Failed to get resource pack list: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get resource pack list",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list": packs,
		},
	})
}

// CreateResourcePack 创建资源包
func (h *AdminHandler) CreateResourcePack(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		TotalCount  int     `json:"total_count" binding:"required"`
		Price       float64 `json:"price" binding:"required"`
		Status      int     `json:"status"`  // 1-上架 0-下架
		Product     string  `json:"product"` // 所属产品/服务标识（如 fv/fv_auth/fv_self/sms）
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	req.Name = adminValidator.SanitizeString(strings.TrimSpace(req.Name))
	if len(req.Name) < 1 || len(req.Name) > 100 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "资源包名称长度必须在1-100个字符之间",
		})
		return
	}
	if req.TotalCount <= 0 || req.TotalCount > 100000 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "认证次数必须大于0且不超过100000",
		})
		return
	}
	if req.Price <= 0 || req.Price > 100000 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "售价必须大于0且不超过100000",
		})
		return
	}
	if req.Status != 0 && req.Status != 1 {
		req.Status = 1
	}

	pack := &model.ResourcePack{
		Name:        req.Name,
		TotalCount:  req.TotalCount,
		Price:       req.Price,
		Status:      req.Status,
		Product:     req.Product,
		Description: adminValidator.SanitizeString(req.Description),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := h.resourcePackRepo.CreatePack(pack); err != nil {
		log.Printf("Failed to create resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to create resource pack",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "pack_create", "pack", pack.ID, fmt.Sprintf("name=%s total_count=%d price=%.2f", pack.Name, pack.TotalCount, pack.Price))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "资源包创建成功",
		"data":    pack,
	})
}

// UpdateResourcePack 更新资源包
func (h *AdminHandler) UpdateResourcePack(c *gin.Context) {
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid pack id",
		})
		return
	}

	pack, err := h.resourcePackRepo.GetPackByID(packID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "resource pack not found",
		})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		TotalCount  int     `json:"total_count"`
		Price       float64 `json:"price"`
		Status      int     `json:"status"`
		Product     string  `json:"product"` // 所属子产品/服务标识
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 仅更新传入的字段
	if req.Name != "" {
		req.Name = adminValidator.SanitizeString(strings.TrimSpace(req.Name))
		if len(req.Name) > 100 {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "资源包名称长度不能超过100个字符",
			})
			return
		}
		pack.Name = req.Name
	}
	if req.TotalCount > 0 {
		if req.TotalCount > 100000 {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "认证次数不能超过100000",
			})
			return
		}
		pack.TotalCount = req.TotalCount
	}
	if req.Price > 0 {
		if req.Price > 100000 {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "售价不能超过100000",
			})
			return
		}
		pack.Price = req.Price
	}
	if req.Status == 0 || req.Status == 1 {
		pack.Status = req.Status
	}
	if req.Product != "" {
		pack.Product = adminValidator.SanitizeString(req.Product)
	}
	pack.Description = adminValidator.SanitizeString(req.Description)
	pack.UpdatedAt = time.Now()

	if err := h.resourcePackRepo.UpdatePack(pack); err != nil {
		log.Printf("Failed to update resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to update resource pack",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "pack_update", "pack", packID, fmt.Sprintf("name=%s total_count=%d price=%.2f status=%d", pack.Name, pack.TotalCount, pack.Price, pack.Status))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "资源包更新成功",
		"data":    pack,
	})
}

// DeleteResourcePack 下架资源包（软删除：status=0，保留已售用户资源包记录）
func (h *AdminHandler) DeleteResourcePack(c *gin.Context) {
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid pack id",
		})
		return
	}

	pack, err := h.resourcePackRepo.GetPackByID(packID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "resource pack not found",
		})
		return
	}

	pack.Status = 0
	pack.UpdatedAt = time.Now()
	if err := h.resourcePackRepo.UpdatePack(pack); err != nil {
		log.Printf("Failed to delete resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to delete resource pack",
		})
		return
	}

	// 记录操作日志
	h.logAdminOperation(c, "pack_delete", "pack", packID, "name="+pack.Name)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "资源包已下架",
	})
}

// ---------- 短信资源包管理（短信库独立表） ----------

// ListSmsResourcePacks 短信资源包列表（管理后台，含已下架；product 为空时返回全部类型）
func (h *AdminHandler) ListSmsResourcePacks(c *gin.Context) {
	product := c.Query("product")
	if product != "" && !isValidSmsPackProduct(product) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid product"})
		return
	}
	packs, err := h.smsResourcePackRepo.ListPacks(nil, product)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to get sms resource packs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": packs}})
}

// isValidSmsPackProduct 短信资源包类型白名单：sms-验证码/通知、sms_marketing-营销
func isValidSmsPackProduct(product string) bool {
	return product == model.ServiceSMS || product == model.ProductSMSMarketing
}

// CreateSmsResourcePack 创建短信资源包（product 区分验证码/通知与营销，默认 sms）
func (h *AdminHandler) CreateSmsResourcePack(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		TotalCount  int     `json:"total_count" binding:"required"`
		Price       float64 `json:"price" binding:"required"`
		Status      int     `json:"status"`
		Product     string  `json:"product"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}
	if req.Product == "" {
		req.Product = model.ServiceSMS
	}
	if !isValidSmsPackProduct(req.Product) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid product"})
		return
	}
	req.Name = adminValidator.SanitizeString(strings.TrimSpace(req.Name))
	if req.Name == "" || len(req.Name) > 100 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "资源包名称长度必须在1-100个字符之间"})
		return
	}
	if req.TotalCount <= 0 || req.TotalCount > 10000000 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "短信条数必须大于0且不超过10000000"})
		return
	}
	if req.Price <= 0 || req.Price > 100000 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "售价必须大于0且不超过100000"})
		return
	}
	if req.Status != 0 && req.Status != 1 {
		req.Status = 1
	}

	pack := &model.SmsResourcePack{
		Name:        req.Name,
		TotalCount:  req.TotalCount,
		Price:       req.Price,
		Status:      req.Status,
		Product:     req.Product,
		Description: adminValidator.SanitizeString(req.Description),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := h.smsResourcePackRepo.CreatePack(pack); err != nil {
		log.Printf("Failed to create sms resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to create sms resource pack"})
		return
	}
	h.logAdminOperation(c, "pack_create", "sms_pack", pack.ID, fmt.Sprintf("name=%s product=%s total_count=%d price=%.2f", pack.Name, pack.Product, pack.TotalCount, pack.Price))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "资源包创建成功", "data": pack})
}

// UpdateSmsResourcePack 更新短信资源包
func (h *AdminHandler) UpdateSmsResourcePack(c *gin.Context) {
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid pack id"})
		return
	}
	pack, err := h.smsResourcePackRepo.GetPackByID(packID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "sms resource pack not found"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		TotalCount  int     `json:"total_count"`
		Price       float64 `json:"price"`
		Status      int     `json:"status"`
		Product     string  `json:"product"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}
	if req.Product != "" {
		if !isValidSmsPackProduct(req.Product) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid product"})
			return
		}
		pack.Product = req.Product
	}
	if req.Name != "" {
		req.Name = adminValidator.SanitizeString(strings.TrimSpace(req.Name))
		if len(req.Name) > 100 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "资源包名称长度不能超过100个字符"})
			return
		}
		pack.Name = req.Name
	}
	if req.TotalCount > 0 {
		if req.TotalCount > 10000000 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "短信条数不能超过10000000"})
			return
		}
		pack.TotalCount = req.TotalCount
	}
	if req.Price > 0 {
		if req.Price > 100000 {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "售价不能超过100000"})
			return
		}
		pack.Price = req.Price
	}
	if req.Status == 0 || req.Status == 1 {
		pack.Status = req.Status
	}
	pack.Description = adminValidator.SanitizeString(req.Description)
	pack.UpdatedAt = time.Now()

	if err := h.smsResourcePackRepo.UpdatePack(pack); err != nil {
		log.Printf("Failed to update sms resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to update sms resource pack"})
		return
	}
	h.logAdminOperation(c, "pack_update", "sms_pack", packID, fmt.Sprintf("name=%s product=%s total_count=%d price=%.2f status=%d", pack.Name, pack.Product, pack.TotalCount, pack.Price, pack.Status))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "资源包更新成功", "data": pack})
}

// DeleteSmsResourcePack 下架短信资源包（软删除：status=0）
func (h *AdminHandler) DeleteSmsResourcePack(c *gin.Context) {
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid pack id"})
		return
	}
	pack, err := h.smsResourcePackRepo.GetPackByID(packID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "sms resource pack not found"})
		return
	}
	pack.Status = 0
	pack.UpdatedAt = time.Now()
	if err := h.smsResourcePackRepo.UpdatePack(pack); err != nil {
		log.Printf("Failed to delete sms resource pack: %v", err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to delete sms resource pack"})
		return
	}
	h.logAdminOperation(c, "pack_delete", "sms_pack", packID, "name="+pack.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "资源包已下架"})
}
