package router

import (
	"net/http"
	"strconv"
	"strings"

	"oemrpa/internal/config"
	"oemrpa/internal/database"
	"oemrpa/internal/handler"
	"oemrpa/internal/middleware"
	"oemrpa/internal/model"
	"oemrpa/internal/redis"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/utils"

	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config) (*gin.Engine, *service.AuthService, *service.BalanceService, *service.NotifyService) {
	r := gin.New()

	// 全局中间件
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.RequestLogger())
	r.Use(middleware.Recovery())

	// 健康检查：/healthz 存活探针（进程存活即通过）；/readyz 就绪探针（依赖的 DB/Redis 可用才通过）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := database.DB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "detail": "database"})
			return
		}
		if err := redis.Client.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "detail": "redis"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Prometheus 指标采集端点
	r.GET("/metrics", metricsHandler)

	// 获取数据库连接
	db := database.DB

	// 初始化 Repository
	userRepo := repository.NewUserRepository(db)
	apiRepo := repository.NewApiKeyRepository(db)
	authRecordRepo := repository.NewAuthRecordRepository(db, model.FvDB)
	paymentRepo := repository.NewPaymentOrderRepository(db)
	billRepo := repository.NewBillRepository(db)
	adminRepo := repository.NewAdminRepository(db)
	kycPersonalRepo := repository.NewKycPersonalRepository(db)
	kybRepo := repository.NewKybEnterpriseRepository(db)
	resourcePackRepo := repository.NewResourcePackRepository(db)
	smsResourcePackRepo := repository.NewSmsResourcePackRepository(db)
	promotionRepo := repository.NewPromotionRepository(db)
	promotionFinanceRepo := repository.NewPromotionFinanceRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	loginLogRepo := repository.NewLoginLogRepository(db)
	smsRepo := repository.NewSMSRepository(db)
	notifyRepo := repository.NewNotifyRecordRepository(db)
	uploadRepo := repository.NewUploadRepository(db)

	// 初始化运行时：从环境变量配置构建第三方业务客户端快照
	rt, err := runtime.New(cfg)
	if err != nil {
		panic("初始化运行时配置失败: " + err.Error())
	}

	// 初始化 Service
	userService := service.NewUserService(userRepo, apiRepo)
	notifyService := service.NewNotifyService(notifyRepo)
	balanceService := service.NewBalanceService(userRepo, billRepo, paymentRepo, resourcePackRepo, smsResourcePackRepo, promotionRepo, db, cfg.PaymentExpireMinutes)

	// 推广服务：平台价取数（产品库配置，无则代码内置默认值）；用户级定向定价在 PromotionService 内另行覆盖
	platformPriceFn := func(svc string) float64 {
		switch svc {
		case model.ServiceFVAuth:
			return rt.FvAuthPrice()
		case model.ServiceFVSelf:
			return rt.FvSelfPrice()
		case model.ServiceSMS:
			if p := rt.SMSPrice(); p > 0 {
				return p
			}
			return 0.05
		case model.ServiceKYCPersonal:
			if cfg.KycPersonalPrice > 0 {
				return cfg.KycPersonalPrice
			}
			return 1.00
		case model.ServiceKYCEnterprise:
			if cfg.KycEnterprisePrice > 0 {
				return cfg.KycEnterprisePrice
			}
			return 2.00
		}
		return 1.00
	}
	// 平台成本单价（产品库配置，无则代码内置默认值）：仅用于按利润计提推广提成（利润 = 实付 − 成本 × 件数）
	productCostFn := func(svc string) float64 {
		switch svc {
		case model.ServiceFVAuth:
			return rt.FvAuthCost()
		case model.ServiceFVSelf:
			return rt.FvSelfCost()
		case model.ServiceSMS:
			return rt.SmsCost()
		case model.ServiceKYCPersonal, model.ServiceKYCEnterprise:
			return 0 // 账户实名由平台承担成本，不参与推广提成
		default:
			// 资源包等历史数据可能只带通用产品标识（fv），按有源核验成本兜底
			return rt.FvAuthCost()
		}
	}
	promotionService := service.NewPromotionService(promotionRepo, userRepo, adminRepo, balanceService, platformPriceFn, productCostFn)

	// 推广收益服务：纯提成模式（收益由提成流水实时汇总），仅负责收益视图与提现
	promotionFinanceService := service.NewPromotionFinanceService(db, promotionFinanceRepo)
	promotionService.SetFinanceService(promotionFinanceService)
	// 提现到余额：复用余额服务的入账口径（传入财务服务的事务，保证「记提现 + 加余额」原子）
	promotionFinanceService.SetBalanceCreditor(balanceService.CreditBalanceTx)
	// 全局推广提成规则（按产品档位：人脸核验 / 短信分别配置，对全部推广商统一生效）：
	// 优先取后台系统设置（可在线改），未配置时回落内置默认值
	promotionService.SetProductRateProvider(func(product string) float64 {
		key := config.SettingKeyAffCommissionRateFV
		if product == model.CommissionProductSMS {
			key = config.SettingKeyAffCommissionRateSMS
		}
		if kv, err := settingRepo.AllSettings(); err == nil {
			if v := strings.TrimSpace(kv[key]); v != "" {
				if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 && n <= 1 {
					return n
				}
			}
		}
		return model.DefaultCommissionRate
	})
	// 单日在线支付限额：优先取后台系统设置（可在线改），未配置回落内置默认值
	balanceService.SetDailyOnlineLimitFn(func() float64 {
		if kv, err := settingRepo.AllSettings(); err == nil {
			if v := strings.TrimSpace(kv[config.SettingKeyPaymentDailyLimit]); v != "" {
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					return n
				}
			}
		}
		return cfg.PaymentDailyLimit
	})
	authService := service.NewAuthService(
		rt.FinAuth,
		rt.FinAuthCfg,
		rt.PlatformFaceId(),
		rt.PlatformOcr(),
		authRecordRepo,
		userRepo,
		apiRepo,
		kycPersonalRepo,
		kybRepo,
		resourcePackRepo,
		balanceService,
		notifyService,
		cfg.MediaDir,
		cfg.FvAuthPrice,
		cfg.FvSelfPrice,
		cfg.KycPersonalPrice,
		cfg.KycEnterprisePrice,
	)
	// 注入推广服务：认证计费按用户适用单价（用户级定向定价优先），并按下级成交额计提提成
	authService.SetPromotionService(promotionService)
	// 注入站点域名解析：实名/人脸核验等对外回跳地址按用户归属的 推广 白标域名下发
	authService.SetSiteHostsResolver(promotionService.SiteHosts)
	// 资源包售价按用户级定价解析（在线组合支付与购买页展示共用）
	balanceService.SetPackPriceResolver(promotionService.PackPrice)
	// 资源包在线支付成交后按下级实付额计提推广提成
	balanceService.SetCommissionAccruer(promotionService.AccruePackCommission)

	// 短信业务服务（唯一上游联麓；未配置时由 handler 层兜底提示）
	smsService := service.NewSmsChannelService(rt.Shlianlu(), rt.SMSPrice(), balanceService, promotionService, smsRepo, apiRepo, notifyService)
	smsHandler := handler.NewSMSHandler(smsService)
	adminSMSHandler := handler.NewAdminSMSHandler(smsService)

	// 初始化 JWT Manager
	jwtManager := utils.NewJWTManager(cfg.JWT.Secret)
	signMgr := utils.NewSignatureManager()

	// 初始化 Handler
	publicHandler := handler.NewPublicHandler(rt, settingRepo, cfg)

	userHandler := handler.NewUserHandler(
		userService,
		rt,
		jwtManager,
		authService,
		loginLogRepo,
		promotionService,
	)

	authHandler := handler.NewAuthHandler(authService, balanceService, promotionService, resourcePackRepo, smsResourcePackRepo, rt)

	promotionHandler := handler.NewPromotionHandler(promotionService, promotionFinanceService, balanceService, rt)
	settingHandler := handler.NewSettingHandler(settingRepo)
	staffHandler := handler.NewAdminStaffHandler(adminRepo, promotionService)

	adminHandler := handler.NewAdminHandler(adminRepo,
		userRepo,
		authRecordRepo,
		paymentRepo,
		billRepo,
		resourcePackRepo,
		smsResourcePackRepo,
		loginLogRepo,
		balanceService,
		authService,
		rt,
		cfg.JWT.AdminSecret,
	)

	callbackHandler := handler.NewCallbackHandler(
		authService,
		balanceService,
		paymentRepo,
		smsService,
		rt,
	)

	dashboardHandler := handler.NewDashboardHandler(db)
	kybAdminHandler := handler.NewKybAdminHandler(authService)
	uploadHandler := handler.NewUploadHandler(cfg, uploadRepo, promotionService.SiteHosts)

	// 路由注册（按前端分为三组）

	// ============ Console 前端路由 ============
	console := r.Group("/console")

	// 上传图片静态访问（后端一级段 = 访问子域：图片域 img.starloft.cn → /img/uploads/yyyyMMdd/xxx.jpg）：
	// 敏感资质图，凭 URL 访问签名或已登录管理员（全部）/上传者本人（控制台用户）可读，其余 403
	uploadAuth := middleware.UploadAuth(cfg.JWT.AdminSecret, cfg.JWT.Secret, uploadRepo)
	r.Group("/img/uploads", uploadAuth).Static("", cfg.UploadDir)

	{
		// 公开接口（无需认证，Console 前端加载验证码配置、渲染支付二维码等）
		console.GET("/config", publicHandler.GetPublicConfig)
		console.GET("/qr", publicHandler.GetQRCode)

		// 用户相关路由（Web前端调用，JWT鉴权）
		console.POST("/send-code", userHandler.SendCode)
		console.POST("/register", userHandler.Register)
		console.POST("/login", userHandler.Login)

		// 需要JWT认证的路由
		auth := console.Group("", middleware.JWTAuth(cfg.JWT.Secret))
		{
			auth.GET("/profile", userHandler.GetProfile)
			auth.GET("/kyc/status", authHandler.GetUserAuthStatus)
			auth.POST("/kyc/sync", authHandler.SyncKycResult)
			auth.POST("/kyc", authHandler.StartAuthForWeb)
			auth.DELETE("/kyc", authHandler.CancelKycRecord)
			// 企业实名（Web，kyb）
			auth.GET("/kyb/status", authHandler.GetKybStatus)
			auth.POST("/kyb", authHandler.StartKybAuthForWeb)
			auth.GET("/records", authHandler.GetUserAuthRecords)
			auth.GET("/stats/calls", authHandler.GetUserAuthCallStats)
			auth.POST("/recharge", authHandler.CreateRecharge)
			auth.GET("/recharge/result", authHandler.GetRechargeResult)
			// 提现（按充值支付订单原路退款）
			auth.GET("/withdraw/refundable", authHandler.GetRefundableOrders) // 可提现（可退款）订单与总额
			auth.POST("/withdraw", authHandler.Withdraw)                      // 发起提现
			// API 密钥（一个账号可创建多把，逐端点授权；创建需已实名）
			auth.GET("/api-keys", userHandler.ListAPIKeys)
			auth.POST("/api-keys", userHandler.CreateAPIKey)
			auth.PUT("/api-keys/:id", userHandler.UpdateAPIKey)
			auth.DELETE("/api-keys/:id", userHandler.DeleteAPIKey)
			auth.POST("/change-password", userHandler.ChangePassword)
			// 人脸核验（Web 用户发起，返回自站链接）
			auth.POST("/fv", authHandler.StartFvAuthForWeb)
			// 短信签名提交（Web 用户）
			auth.POST("/sms/sign", smsHandler.SubmitSign)
			auth.GET("/sms/signs", smsHandler.ListSigns)
			auth.GET("/sms/signs/:id", smsHandler.GetSignDetail)          // 签名详情（修改页回填）
			auth.POST("/sms/signs/:id/query", smsHandler.QuerySignStatus) // 手动查询签名审核状态（上游核对+回写）
			auth.PUT("/sms/signs/:id", smsHandler.UpdateSign)             // 修改签名（创建新签名+删除旧签名，复用本地记录）
			auth.POST("/sms/templates", smsHandler.CreateTemplate)
			auth.PUT("/sms/templates/:id", smsHandler.UpdateTemplate) // 修改模板（重新报备上游并重置待审核）
			auth.GET("/sms/templates", smsHandler.ListTemplates)
			auth.GET("/sms/templates/:id", smsHandler.GetTemplateDetail) // 模板详情（修改页回填）
			auth.POST("/sms/templates/:id/template-id", smsHandler.FillTemplateID)
			auth.POST("/sms/templates/:id/query", smsHandler.QueryTemplateStatus) // 手动查询模板审核状态（上游核对+回写）
			auth.GET("/sms/records", smsHandler.ListSendRecords)
			auth.GET("/sms/stats", smsHandler.SMSStats)
			// 在线发送短信（登录用户直接发送，按自己账号的资源包/余额计费）
			auth.POST("/sms/send", smsHandler.SendSMSForWeb)
			// 短信资源包（短信库独立表；短信单一产品，无 product 类型）
			auth.GET("/sms/packs", authHandler.ListSmsResourcePacks)                  // 在售短信资源包列表
			auth.POST("/sms/packs/:id/purchase", authHandler.PurchaseSmsResourcePack) // 使用余额购买短信资源包
			auth.GET("/sms/packs/mine", authHandler.MySmsResourcePacks)               // 我的短信资源包
			// 用户文件上传（营业执照/身份证等图片）
			auth.POST("/upload", uploadHandler.UploadFile)
			// 资源包（余额购买 / 在线组合支付）
			auth.GET("/packs", authHandler.ListResourcePacks)                   // 在售资源包列表
			auth.POST("/packs/:id/purchase", authHandler.PurchaseResourcePack)  // 使用余额购买资源包
			auth.POST("/packs/:id/pay", authHandler.PurchaseResourcePackOnline) // 在线购买（余额+支付宝组合支付）
			auth.GET("/packs/mine", authHandler.MyResourcePacks)                // 我的资源包

			// 推广（控制台 /promotions）：打开推广页即自动开通推广码，按下级成交额获取提成收益
			auth.GET("/promotions/me", promotionHandler.Me)                    // 我的推广概览（含推广码/推广链接，无码则自动生成）
			auth.GET("/promotions/users", promotionHandler.SubUsers)           // 我的推广用户
			auth.GET("/promotions/finance", promotionHandler.Finance)          // 收益概览（累计/已提现/可提现）
			auth.GET("/promotions/finance-logs", promotionHandler.FinanceLogs) // 提成流水
			auth.GET("/promotions/withdraws", promotionHandler.Withdraws)      // 我的提现申请
			auth.POST("/promotions/withdraw", promotionHandler.WithdrawApply)  // 发起提现申请（提现到余额，免手续费）
		}
	}

	// ============ Admin 前端路由 ============
	admin := r.Group("/admin")
	{
		// 管理员登录 - 限流每分钟5次
		admin.POST("/login", middleware.RateLimiterForIP(5), adminHandler.AdminLogin)

		// 需要管理员JWT认证的路由 - 每分钟100次；权限守卫按路由前缀逐项判权（fail-closed）
		adminAuth := admin.Group("", middleware.JWTAuth(cfg.JWT.AdminSecret), middleware.AdminPermissionGuard(adminRepo), middleware.RateLimiter(100))
		{
			// 当前登录账号信息（含权限清单；无权限账号亦可访问，用于构建菜单）
			adminAuth.GET("/me", staffHandler.Me)

			// 员工管理（后台账号增改、权限勾选、启停、重置密码）
			adminAuth.GET("/admins", staffHandler.List)
			adminAuth.POST("/admins", staffHandler.Create)
			adminAuth.PUT("/admins/:id", staffHandler.Update)
			adminAuth.PUT("/admins/:id/password", staffHandler.ResetPassword)

			// 销售业绩（员工/销售本人或超管按 staff_id 查看）
			adminAuth.GET("/sales/summary", promotionHandler.AdminSalesSummary)
			adminAuth.GET("/sales/commissions", promotionHandler.AdminSalesCommissions)
			adminAuth.GET("/sales/users", promotionHandler.AdminSalesUsers)

			// 用户管理
			adminAuth.GET("/users", adminHandler.GetUserList)
			adminAuth.GET("/users/:id", adminHandler.GetUserDetail)

			// 企业实名记录（kyb）与个人实名记录（kyc）管理
			adminAuth.GET("/kyb", kybAdminHandler.ListKybRecords)
			adminAuth.POST("/kyb/verify", kybAdminHandler.VerifyKyb)
			adminAuth.GET("/kyc", kybAdminHandler.ListKycPersonalRecords)
			adminAuth.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
			// 账户实名两档的用户定向定价（覆盖平台价；人脸核验/短信的定向定价见产品配置页接口）
			adminAuth.GET("/users/:id/prices", promotionHandler.AdminUserPrices)
			adminAuth.PUT("/users/:id/prices", promotionHandler.AdminSetUserPrices)

			// 推广管理：提成记录 / 提现审核
			adminAuth.GET("/promotions/commissions", promotionHandler.AdminCommissions) // 提成记录（用户型推广 + 员工销售）
			adminAuth.GET("/promotion-withdraws", promotionHandler.AdminWithdraws)
			adminAuth.PUT("/promotion-withdraws/:id/review", promotionHandler.AdminReviewWithdraw)
			adminAuth.PUT("/promotion-withdraws/:id/paid", promotionHandler.AdminMarkWithdrawPaid)

			// 平台配置：第三方密钥（系统库 setting）与产品配置（各产品库 product_config）
			adminAuth.GET("/settings", settingHandler.GetSettings)
			adminAuth.PUT("/settings", settingHandler.UpsertSetting)
			adminAuth.DELETE("/settings", settingHandler.DeleteSetting)
			adminAuth.GET("/product-config", settingHandler.GetProductConfigs)
			adminAuth.PUT("/product-config", settingHandler.UpsertProductConfig)
			adminAuth.DELETE("/product-config", settingHandler.DeleteProductConfig)
			// 产品维度的用户定向定价（覆盖价存各产品库；权限随 /product-config 前缀 + handler 内按 product 二次判定）
			adminAuth.GET("/product-config/users", promotionHandler.AdminProductPriceUsers)          // 选人：按手机号/用户名检索
			adminAuth.GET("/product-config/user-prices", promotionHandler.AdminProductUserPrices)    // 平台价 / 生效价 / 自定义价
			adminAuth.PUT("/product-config/user-prices", promotionHandler.AdminSetProductUserPrices) // 保存（批量写入 + 差异删除）
			adminAuth.POST("/users/:id/reset-kyc-free", adminHandler.ResetUserKycFree)               // 重置用户免费实名次数
			adminAuth.POST("/users/:id/recharge", adminHandler.RechargeUserBalance)                  // 人工充值（需银行流水单号）
			adminAuth.GET("/users/:id/finance/stats", adminHandler.GetUserFinanceStats)              // 用户财务统计
			adminAuth.GET("/users/:id/balance-logs", adminHandler.GetUserBalanceLogs)                // 用户余额流水
			adminAuth.GET("/users/:id/auth-records", adminHandler.GetUserAuthRecords)                // 用户认证记录
			adminAuth.POST("/users/register", adminHandler.ManualRegisterUser)                       // 人工注册账号

			// 资源包管理
			adminAuth.GET("/packs", adminHandler.GetResourcePackList)
			adminAuth.POST("/packs", adminHandler.CreateResourcePack)
			adminAuth.POST("/packs/grant-test", adminHandler.GrantFvTestPack) // 发放人脸核验测试资源包（有源 fv_auth / 无源 fv_self）
			adminAuth.PUT("/packs/:id", adminHandler.UpdateResourcePack)
			adminAuth.DELETE("/packs/:id", adminHandler.DeleteResourcePack)
			// 短信资源包管理（短信库独立表）
			adminAuth.GET("/sms/packs", adminHandler.ListSmsResourcePacks)
			adminAuth.POST("/sms/packs", adminHandler.CreateSmsResourcePack)
			adminAuth.POST("/sms/packs/grant-test", adminHandler.GrantSmsTestPack) // 发放短信测试资源包
			adminAuth.PUT("/sms/packs/:id", adminHandler.UpdateSmsResourcePack)
			adminAuth.DELETE("/sms/packs/:id", adminHandler.DeleteSmsResourcePack)

			// 订单管理
			adminAuth.GET("/records", adminHandler.GetAuthRecordList)
			adminAuth.GET("/records/recent", adminHandler.GetRecentAuthRecords)
			adminAuth.GET("/records/:id", adminHandler.GetAuthRecordDetail)
			// 手动查询认证记录的上游结果（调上游核对并回写本地）
			adminAuth.POST("/records/:id/query-result", adminHandler.QueryAuthRecordResult)
			adminAuth.GET("/payments", adminHandler.GetPaymentOrderList)
			// 手动查询支付结果（向渠道查询真实状态并落地，非强制入账）
			adminAuth.POST("/payments/:id/query-result", adminHandler.QueryPaymentResult)
			// 统一账单
			adminAuth.GET("/bills", adminHandler.GetBills)

			// 管理员修改密码
			adminAuth.POST("/change-password", adminHandler.ChangePassword)

			// 短信签名/模板后台人工审核
			adminAuth.GET("/sms/signs", adminSMSHandler.ListSigns)
			adminAuth.POST("/sms/signs/:id/review", adminSMSHandler.ReviewSign)
			adminAuth.POST("/sms/signs/:id/query", adminSMSHandler.QuerySignStatus)                 // 查询审核结果（调上游核对并回写）
			adminAuth.PUT("/sms/signs/:id/result-message", adminSMSHandler.UpdateSignResultMessage) // 修改审核结果/失败原因文本
			adminAuth.PUT("/sms/signs/:id/public", adminSMSHandler.SetSignPublic)                   // 设为公共/私有（仅改可见性）
			adminAuth.GET("/sms/templates", adminSMSHandler.ListTemplates)
			adminAuth.POST("/sms/templates/:id/review", adminSMSHandler.ReviewTemplate)
			adminAuth.POST("/sms/templates/:id/query", adminSMSHandler.QueryTemplateStatus) // 查询审核结果（调上游核对并回写）
			// 短信发送记录/统计/回复（管理端全量，含用户手机号）
			adminAuth.GET("/sms/records", adminSMSHandler.ListSendRecords)
			adminAuth.GET("/sms/stats", adminSMSHandler.SMSStats)
			adminAuth.GET("/sms/replies", adminSMSHandler.ListReplies)

			// Dashboard数据
			adminAuth.GET("/dashboard", dashboardHandler.GetDashboard)

			// 财务统计
			adminAuth.GET("/finance/summary", dashboardHandler.GetFinanceSummary)
			adminAuth.GET("/finance/daily", dashboardHandler.GetDailyFinanceStats)

			// 数据统计
			adminAuth.GET("/stats/overview", adminHandler.GetStatisticsOverview)
			adminAuth.GET("/stats/orders", adminHandler.GetOrderStatistics)
			adminAuth.GET("/stats/revenue", adminHandler.GetIncomeStatistics)
		}
	}

	// ============ 外部 API 路由（面向下游/插件；后端一级段取子域标签 api，用户经 api.starloft.cn/v1/* 访问） ============
	api := r.Group("/api/v1")
	{
		// 人脸核验 FV API：有源（auth）/ 无源（self）；按端点逐个判权
		fv := api.Group("/fv",
			middleware.APIKeyMiddleware(userRepo, apiRepo, signMgr),
		)
		{
			fv.POST("/auth", middleware.ServiceAccessGuard(model.APIFvAuth), authHandler.StartFvAuthForAPI(model.ServiceFVAuth))
			fv.POST("/self", middleware.ServiceAccessGuard(model.APIFvSelf), authHandler.StartFvAuthForAPI(model.ServiceFVSelf))
			// 活体最佳图领取：认证成功后 24 小时内、每笔订单仅一次
			fv.POST("/best-img", middleware.ServiceAccessGuard(model.APIFvBestImg), authHandler.GetFvBestImg)
			// 认证媒体下载：认证成功后自动保存的照片/视频（保存 30 天，base64 返回）
			fv.POST("/media", middleware.ServiceAccessGuard(model.APIFvMedia), authHandler.GetFvMedia)
			// 按业务流水号查询认证订单状态（下游轮询/结果校对）
			fv.POST("/result", middleware.ServiceAccessGuard(model.APIFvResult), authHandler.QueryFvResult)
		}

		// 短信（API Key 鉴权，按端点逐个判权；个人与企业实名均可用）
		// 发送 + 平台模板型模板管理（供魔方财务/v10 插件调用）
		sms := api.Group("/sms",
			middleware.APIKeyMiddleware(userRepo, apiRepo, signMgr),
		)
		{
			sms.POST("/send",
				middleware.ServiceAccessGuard(model.APISmsSend),
				smsHandler.SendSMSForAPI,
			)
			// 创建短信签名（资质图片以 URL 传入，平台不处理图片直接透传上游）
			sms.POST("/signs",
				middleware.ServiceAccessGuard(model.APISmsSign),
				smsHandler.CreateSignForAPI,
			)
			sms.POST("/templates",
				middleware.ServiceAccessGuard(model.APISmsTemplateCreate),
				smsHandler.CreateTemplateForAPI,
			)
			sms.GET("/templates/:id",
				middleware.ServiceAccessGuard(model.APISmsTemplateGet),
				smsHandler.GetTemplateForAPI,
			)
			sms.PUT("/templates/:id",
				middleware.ServiceAccessGuard(model.APISmsTemplateUpdate),
				smsHandler.UpdateTemplateForAPI,
			)
			sms.DELETE("/templates/:id",
				middleware.ServiceAccessGuard(model.APISmsTemplateDelete),
				smsHandler.DeleteTemplateForAPI,
			)
			// 查询短信发送回执（按上游 taskId，须 API Key 鉴权）
			sms.POST("/report",
				middleware.ServiceAccessGuard(model.APISmsReport),
				smsHandler.QuerySmsReport,
			)
			// 查询短信上行回复（按日期拉取同步 + 本地查询，须 API Key 鉴权）
			sms.POST("/replies",
				middleware.ServiceAccessGuard(model.APISmsReplies),
				smsHandler.QueryReplies,
			)
		}

		// 回调接口（无需认证，按来源 IP 白名单校验：CALLBACK_TRUST_IPS，未配置时放行）
		callback := api.Group("/callback", middleware.CallbackGuard(cfg.CallbackTrustIPs))
		{
			callback.POST("/finauth", callbackHandler.FinAuthCallback)
			callback.POST("/alipay", callbackHandler.AlipayCallback)
			callback.POST("/wechat", callbackHandler.WechatCallback)
			// 短信回执推送（联麓「发送状态推送地址」，无签名验签，返回 llcode=0 防重推）
			callback.POST("/sms-report", callbackHandler.SmsReportCallback)
			// 短信签名状态推送（联麓「签名状态推送地址」，无签名验签）
			callback.POST("/sms-sign-status", callbackHandler.SmsSignStatusCallback)
			// 短信模板状态推送（联麓「模板状态推送地址」，无签名验签）
			callback.POST("/sms-template-status", callbackHandler.SmsTemplateStatusCallback)
			// 短信回复推送（联麓「短信回复推送地址」，无签名验签，返回 llcode=0 防重推）
			callback.POST("/sms-reply", callbackHandler.SmsReplyCallback)
		}
	}

	// 人脸核验自站 return 回调（公开端点，无需 API Key，用户经 api.starloft.cn/v1/fv/return 访问）：
	// 用户核身后先回本平台校对一次，再 302 到下游 return_url
	r.GET("/api/v1/fv/return", authHandler.HandleFvReturn)
	r.POST("/api/v1/fv/return", authHandler.HandleFvReturn)

	// FV 承接页订单状态查询（公开端点，无需登录，一级段取子域标签 service，
	// 用户经 service.starloft.cn/service/fv/status 访问）：承接页轮询判断订单是否已结束
	r.GET("/service/fv/status", authHandler.GetFvPageStatus)

	// 权限规则覆盖自检：新增后台路由却未登记权限时打告警日志（不阻断启动）
	middleware.CheckAdminRouteCoverage(r.Routes())

	return r, authService, balanceService, notifyService
}
