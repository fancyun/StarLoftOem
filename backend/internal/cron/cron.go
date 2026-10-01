package cron

import (
	"log"

	"github.com/robfig/cron/v3"

	"oemrpa/internal/service"
	"oemrpa/internal/upstream"
)

// CronManager 定时任务管理器
type CronManager struct {
	cron           *cron.Cron
	authService    *service.AuthService
	balanceService *service.BalanceService
	notifyService  *service.NotifyService
	alipay         *upstream.AlipayClient
	wechatPay      *upstream.WechatPayClient
}

// NewCronManager 创建定时任务管理器
func NewCronManager(authService *service.AuthService, balanceService *service.BalanceService, notifyService *service.NotifyService, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) *CronManager {
	return &CronManager{
		cron:           cron.New(cron.WithSeconds()),
		authService:    authService,
		balanceService: balanceService,
		notifyService:  notifyService,
		alipay:         alipay,
		wechatPay:      wechatPay,
	}
}

// Start 启动定时任务
func (m *CronManager) Start() error {
	// 每20分钟同步一次处理中记录，根据上游返回结果处理退款
	// （上游核身有效期为 60 分钟，20 分钟一次的补查频率足够且更省上游 get_result 配额）
	_, err := m.cron.AddFunc("0 */20 * * * *", m.syncPendingRecords)
	if err != nil {
		return err
	}

	// 每5分钟终结一次核身超时未完成的认证订单（置「超时结束」并退还已扣费用，幂等）
	_, err = m.cron.AddFunc("0 */5 * * * *", m.expirePendingRecords)
	if err != nil {
		return err
	}

	// 每分钟执行一次超时撤回：扫描过期待支付订单，向渠道关单撤回并关闭本地单
	// （资源包订单额外退还余额支付部分，幂等）
	_, err = m.cron.AddFunc("0 * * * * *", m.releaseExpiredPaymentOrders)
	if err != nil {
		return err
	}

	// 每分钟查询一次未支付的支付订单真实状态（向支付宝/微信补账或关闭，替代原每日对账）
	_, err = m.cron.AddFunc("0 */1 * * * *", m.reconcilePaymentOrders)
	if err != nil {
		return err
	}

	// 每日 3 点清理已过期的认证媒体文件（FV 照片/视频保存 30 天，过期删除）
	_, err = m.cron.AddFunc("0 0 3 * * *", m.cleanExpiredMedia)
	if err != nil {
		return err
	}

	// 每5分钟补推下游通知（FV结果/短信回执/短信回复，指数退避重试）
	_, err = m.cron.AddFunc("0 */5 * * * *", m.retryNotifyRecords)
	if err != nil {
		return err
	}

	// 每日 3:30 日终对账：渠道已支付流水 vs 本地账单入账差异告警
	_, err = m.cron.AddFunc("0 30 3 * * *", m.dailyReconcile)
	if err != nil {
		return err
	}

	// 每30分钟业务告警巡检：认证记录长期 pending、通知重试受阻
	_, err = m.cron.AddFunc("0 */30 * * * *", m.businessAlerts)
	if err != nil {
		return err
	}

	m.cron.Start()
	log.Println("定时任务已启动")

	return nil
}

// Stop 停止定时任务
func (m *CronManager) Stop() {
	m.cron.Stop()
	log.Println("定时任务已停止")
}

// syncPendingRecords 同步处理中记录并根据上游结果处理退款
func (m *CronManager) syncPendingRecords() {
	log.Println("开始同步处理中记录...")

	err := m.authService.SyncPendingRecords()
	if err != nil {
		log.Printf("同步处理中记录失败: %v", err)
		return
	}

	log.Println("处理中记录同步完成")
}

// expirePendingRecords 终结核身超时未完成的认证订单（退还已扣费用；仅错误时记录日志）
func (m *CronManager) expirePendingRecords() {
	if err := m.authService.ExpirePendingRecords(); err != nil {
		log.Printf("终结超时认证订单失败: %v", err)
	}
}

// releaseExpiredPaymentOrders 超时撤回支付（每分钟执行；仅错误时记录日志）
func (m *CronManager) releaseExpiredPaymentOrders() {
	if err := m.balanceService.ReleaseExpiredPaymentOrders(m.alipay, m.wechatPay); err != nil {
		log.Printf("超时撤回支付失败: %v", err)
	}
}

// reconcilePaymentOrders 每分钟向支付宝/微信查询未支付订单的真实状态并补账/关闭（幂等）
func (m *CronManager) reconcilePaymentOrders() {
	m.balanceService.ReconcilePaymentOrders(m.alipay, m.wechatPay)
}

// cleanExpiredMedia 每日清理过期的认证媒体文件（FV 照片/视频，保存 30 天）
func (m *CronManager) cleanExpiredMedia() {
	m.authService.CleanExpiredMedia()
}

// retryNotifyRecords 补推失败的下游通知（FV结果/短信回执/短信回复）
func (m *CronManager) retryNotifyRecords() {
	m.notifyService.RetryDue()
}

// dailyReconcile 日终对账：渠道已支付流水 vs 本地账单入账差异
func (m *CronManager) dailyReconcile() {
	m.balanceService.DailyReconcile()
}

// businessAlerts 业务告警巡检：认证记录长期 pending、通知重试受阻
func (m *CronManager) businessAlerts() {
	m.balanceService.CheckBusinessAlerts()
}
