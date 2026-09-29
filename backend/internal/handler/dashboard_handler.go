package handler

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/model"
)

type DashboardHandler struct {
	db *sql.DB
}

func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

// ---------- 认证记录统计（人脸核验 oem_fv 库） ----------
// 认证记录仅存储于人脸核验库 oem_fv。

// countAuthRecords 对人脸核验库执行 COUNT；where 为条件片段（如 "WHERE status = 2"），args 为对应占位参数
func (h *DashboardHandler) countAuthRecords(where string, args ...interface{}) (int64, error) {
	var n int64
	q := "SELECT COUNT(*) FROM " + model.FvDB + ".auth_record " + where
	if err := h.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// sumAuthRecordsIncome 对人脸核验库求和已核验收入，返回 SUM(cost) 与 COUNT(*)；where 为条件片段
func (h *DashboardHandler) sumAuthRecordsIncome(where string, args ...interface{}) (float64, int64, error) {
	var amount float64
	var count int64
	q := "SELECT COALESCE(SUM(cost), 0), COUNT(*) FROM " + model.FvDB + ".auth_record " + where
	if err := h.db.QueryRow(q, args...).Scan(&amount, &count); err != nil {
		return 0, 0, err
	}
	return amount, count, nil
}

// GetDashboard 获取Dashboard统计数据 (新接口名称)
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	h.GetDashboardStats(c)
}

// GetFinanceSummary 获取财务汇总 (新接口名称)
func (h *DashboardHandler) GetFinanceSummary(c *gin.Context) {
	h.GetFinanceStats(c)
}

// GetDailyFinanceStats 获取每日财务统计
func (h *DashboardHandler) GetDailyFinanceStats(c *gin.Context) {
	h.GetFinanceStats(c)
}

// GetDashboardStats 获取Dashboard统计数据
func (h *DashboardHandler) GetDashboardStats(c *gin.Context) {
	// 1. 用户统计
	var totalUsers, activeUsers, kycVerifiedUsers int64
	err := h.db.QueryRow("SELECT COUNT(*) FROM " + model.SysDB + ".user").Scan(&totalUsers)
	if err != nil {
		log.Printf("Failed to count total users: %v", err)
	}

	err = h.db.QueryRow("SELECT COUNT(*) FROM " + model.SysDB + ".user WHERE status = 1").Scan(&activeUsers)
	if err != nil {
		log.Printf("Failed to count active users: %v", err)
	}

	err = h.db.QueryRow("SELECT COUNT(*) FROM " + model.SysDB + ".user WHERE realname_status > 0").Scan(&kycVerifiedUsers)
	if err != nil {
		log.Printf("Failed to count KYC verified users: %v", err)
	}

	// 2. 今日新增用户
	today := time.Now().Format("2006-01-02")
	var todayNewUsers int64
	err = h.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".user WHERE DATE(created_at) = ?", today).Scan(&todayNewUsers)
	if err != nil {
		log.Printf("Failed to count today new users: %v", err)
	}

	// 3. 订单统计（FV 库）
	var totalAuthRecords, todayAuthRecords, successAuthRecords int64
	if v, err := h.countAuthRecords(""); err == nil {
		totalAuthRecords = v
	} else {
		log.Printf("Failed to count total auth orders: %v", err)
	}

	if v, err := h.countAuthRecords("WHERE DATE(created_at) = ?", today); err == nil {
		todayAuthRecords = v
	} else {
		log.Printf("Failed to count today auth orders: %v", err)
	}

	if v, err := h.countAuthRecords("WHERE status = 2"); err == nil {
		successAuthRecords = v
	} else {
		log.Printf("Failed to count success auth orders: %v", err)
	}

	// 4. 充值统计
	var totalRechargeAmount, todayRechargeAmount float64
	var totalRechargeOrders, todayRechargeOrders int64

	err = h.db.QueryRow("SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM payment_order WHERE status = 1").Scan(&totalRechargeAmount, &totalRechargeOrders)
	if err != nil {
		log.Printf("Failed to count total recharge: %v", err)
	}

	err = h.db.QueryRow("SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM payment_order WHERE status = 1 AND DATE(paid_at) = ?", today).Scan(&todayRechargeAmount, &todayRechargeOrders)
	if err != nil {
		log.Printf("Failed to count today recharge: %v", err)
	}

	// 5. 消费统计
	var totalConsumeAmount, todayConsumeAmount float64
	err = h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM " + model.SysDB + ".bill WHERE bill_type = 2").Scan(&totalConsumeAmount)
	if err != nil {
		log.Printf("Failed to count total consume: %v", err)
	}

	err = h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM "+model.SysDB+".bill WHERE bill_type = 2 AND DATE(created_at) = ?", today).Scan(&todayConsumeAmount)
	if err != nil {
		log.Printf("Failed to count today consume: %v", err)
	}

	// 6. 系统余额统计
	var totalUserBalance float64
	err = h.db.QueryRow("SELECT COALESCE(SUM(balance), 0) FROM " + model.SysDB + ".user").Scan(&totalUserBalance)
	if err != nil {
		log.Printf("Failed to count total user balance: %v", err)
	}

	// 7. 最近7天的趋势数据
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	type DayStats struct {
		Date           string  `json:"date"`
		NewUsers       int64   `json:"new_users"`
		AuthRecords    int64   `json:"auth_records"`
		RechargeAmount float64 `json:"recharge_amount"`
		ConsumeAmount  float64 `json:"consume_amount"`
	}

	rows, err := h.db.Query(`
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as new_users
		FROM `+model.SysDB+`.user 
		WHERE DATE(created_at) >= ? 
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, sevenDaysAgo)

	dayStatsMap := make(map[string]*DayStats)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date string
			var count int64
			if err := rows.Scan(&date, &count); err == nil {
				dayStatsMap[date] = &DayStats{
					Date:     date,
					NewUsers: count,
				}
			}
		}
	}

	// 补充其他统计数据
	for date := range dayStatsMap {
		// 认证记录（FV 库）
		if v, err := h.countAuthRecords("WHERE DATE(created_at) = ?", date); err == nil {
			dayStatsMap[date].AuthRecords = v
		}

		// 充值金额
		h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM payment_order WHERE status = 1 AND DATE(paid_at) = ?", date).Scan(&dayStatsMap[date].RechargeAmount)

		// 消费金额（统一账单 bill_type=2）
		h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM "+model.SysDB+".bill WHERE bill_type = 2 AND DATE(created_at) = ?", date).Scan(&dayStatsMap[date].ConsumeAmount)
	}

	// 转换为数组
	trendData := make([]*DayStats, 0, len(dayStatsMap))
	for _, stats := range dayStatsMap {
		trendData = append(trendData, stats)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"users": gin.H{
				"total":        totalUsers,
				"active":       activeUsers,
				"kyc_verified": kycVerifiedUsers,
				"today_new":    todayNewUsers,
			},
			"records": gin.H{
				"total_auth":   totalAuthRecords,
				"today_auth":   todayAuthRecords,
				"success_auth": successAuthRecords,
			},
			"finance": gin.H{
				"total_recharge_amount": totalRechargeAmount,
				"total_recharge_orders": totalRechargeOrders,
				"today_recharge_amount": todayRechargeAmount,
				"today_recharge_orders": todayRechargeOrders,
				"total_consume_amount":  totalConsumeAmount,
				"today_consume_amount":  todayConsumeAmount,
				"total_user_balance":    totalUserBalance,
			},
			"trend": trendData,
		},
	})
}

// GetFinanceStats 获取财务统计
func (h *DashboardHandler) GetFinanceStats(c *gin.Context) {
	// 获取查询参数
	startDate := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	endDate := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	// 1. 充值统计
	type RechargeStats struct {
		TotalAmount  float64 `json:"total_amount"`
		TotalOrders  int64   `json:"total_orders"`
		AlipayAmount float64 `json:"alipay_amount"`
	}

	rechargeStats := &RechargeStats{}
	err := h.db.QueryRow(`
		SELECT 
			COALESCE(SUM(amount), 0) as total_amount,
			COUNT(*) as total_orders
		FROM payment_order 
		WHERE status = 1 AND DATE(paid_at) BETWEEN ? AND ?
	`, startDate, endDate).Scan(&rechargeStats.TotalAmount, &rechargeStats.TotalOrders)

	if err != nil {
		log.Printf("Failed to query recharge stats: %v", err)
	}

	// 按渠道统计
	h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM payment_order WHERE status = 1 AND channel = 'alipay' AND DATE(paid_at) BETWEEN ? AND ?", startDate, endDate).Scan(&rechargeStats.AlipayAmount)

	// 2. 消费统计
	type ConsumeStats struct {
		TotalAmount      float64 `json:"total_amount"`
		TotalCount       int64   `json:"total_count"`
		KYCConsumeAmount float64 `json:"kyc_consume_amount"`
		KYCConsumeCount  int64   `json:"kyc_consume_count"`
	}

	consumeStats := &ConsumeStats{}
	err = h.db.QueryRow(`
		SELECT 
			COALESCE(SUM(amount), 0) as total_amount,
			COUNT(*) as total_count
		FROM `+model.SysDB+`.bill 
		WHERE bill_type = 2 AND DATE(created_at) BETWEEN ? AND ?
	`, startDate, endDate).Scan(&consumeStats.TotalAmount, &consumeStats.TotalCount)

	if err != nil {
		log.Printf("Failed to query consume stats: %v", err)
	}

	// 认证消费（FV 库）
	if amount, count, err := h.sumAuthRecordsIncome("WHERE status = 2 AND DATE(finished_at) BETWEEN ? AND ?", startDate, endDate); err == nil {
		consumeStats.KYCConsumeAmount = amount
		consumeStats.KYCConsumeCount = count
	}

	// 3. 每日统计
	rows, err := h.db.Query(`
		SELECT 
			DATE(paid_at) as date,
			COALESCE(SUM(amount), 0) as amount,
			COUNT(*) as count
		FROM payment_order 
		WHERE status = 1 AND DATE(paid_at) BETWEEN ? AND ?
		GROUP BY DATE(paid_at)
		ORDER BY date ASC
	`, startDate, endDate)

	type DailyStats struct {
		Date           string  `json:"date"`
		RechargeAmount float64 `json:"recharge_amount"`
		ConsumeAmount  float64 `json:"consume_amount"`
	}

	dailyStats := make([]*DailyStats, 0)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date string
			var amount float64
			var count int64
			if err := rows.Scan(&date, &amount, &count); err == nil {
				daily := &DailyStats{
					Date:           date,
					RechargeAmount: amount,
				}

				// 查询当日消费
				h.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM "+model.SysDB+".bill WHERE bill_type = 2 AND DATE(created_at) = ?", date).Scan(&daily.ConsumeAmount)

				dailyStats = append(dailyStats, daily)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"date_range": gin.H{
				"start_date": startDate,
				"end_date":   endDate,
			},
			"recharge":    rechargeStats,
			"consume":     consumeStats,
			"daily_stats": dailyStats,
		},
	})
}
