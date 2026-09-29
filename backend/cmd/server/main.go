package main

import (
	"log"
	"oemrpa/internal/config"
	"oemrpa/internal/cron"
	"oemrpa/internal/database"
	"oemrpa/internal/model"
	"oemrpa/internal/redis"
	"oemrpa/internal/repository"
	"oemrpa/internal/router"
	"oemrpa/internal/site"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

// applyDBConfig 用数据库中的配置覆盖代码内置默认值：非密钥业务配置取系统库 setting，平台单价与成本取各产品库 product_config。
// 数据库连接/Redis/JWT/数据加密等自举密钥不参与覆盖。
func applyDBConfig(cfg *config.Config) error {
	repo := repository.NewSettingRepository(database.DB)

	sysKV, err := repo.AllSettings()
	if err != nil {
		return err
	}
	fvKV, err := repo.ProductConfigs(model.FvDB)
	if err != nil {
		return err
	}
	smsKV, err := repo.ProductConfigs(model.SmsDB)
	if err != nil {
		return err
	}

	config.ApplySettingOverrides(cfg, sysKV, fvKV, smsKV)
	if msg := config.FormatMissingKeys(cfg.MissingThirdPartyKeys()); msg != "" {
		log.Print(msg)
	}

	// 预置配置目录：表内缺失的键按当前生效值写入（已有值不覆盖），
	// 后台「系统设置 / 产品配置」打开即是完整清单，直接改值即可
	if err := seedSettingCatalog(cfg, repo); err != nil {
		log.Printf("预置配置目录失败（可稍后在后台手工补填）: %v", err)
	}
	return nil
}

// seedSettingCatalog 幂等预置系统配置与产品库配置的键（INSERT IGNORE，不覆盖已修改的值）
func seedSettingCatalog(cfg *config.Config, repo *repository.SettingRepository) error {
	values := cfg.SettingValues()
	for _, spec := range config.SettingCatalog() {
		if err := repo.InsertSettingIfAbsent(spec.Key, values[spec.Key], spec.Category, spec.Remark); err != nil {
			return err
		}
	}

	fvValues := cfg.FvProductConfigValues()
	for _, spec := range config.FvProductConfigCatalog() {
		if err := repo.InsertProductConfigIfAbsent(model.FvDB, spec.Key, fvValues[spec.Key], spec.Remark); err != nil {
			return err
		}
	}

	smsValues := cfg.SmsProductConfigValues()
	for _, spec := range config.SmsProductConfigCatalog() {
		if err := repo.InsertProductConfigIfAbsent(model.SmsDB, spec.Key, smsValues[spec.Key], spec.Remark); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// 加载配置（从环境变量）
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化敏感字段加密器（AES-256-GCM，密钥来自 DATA_ENCRYPT_KEY）
	if err := utils.InitCrypto(cfg.DataEncryptKey); err != nil {
		log.Fatalf("初始化数据加密失败: %v", err)
	}

	// 初始化上传图片访问签名器（HMAC 密钥取自 JWT_SECRET）
	if err := utils.InitUploadSigner(cfg.JWT.Secret); err != nil {
		log.Fatalf("初始化图片访问签名失败: %v", err)
	}

	// 初始化文件日志（写入 /app/logs，可按 LOG_DIR 覆盖）
	if err := utils.InitLoggers(cfg.Log.Dir); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	// 初始化数据库
	if err := database.Init(cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 数据库配置覆盖内置默认值：非密钥业务配置取系统库 setting，平台单价与成本取各产品库 product_config；
	// 读取失败时仅记日志，沿用 .env 与内置默认值继续启动
	if err := applyDBConfig(cfg); err != nil {
		log.Printf("加载数据库配置失败，沿用内置默认值: %v", err)
	}

	// 站点主域：由后台配置注入（未配置时沿用代码内置默认值）。
	// 各站点域名与对外回调/跳转地址均据此拼装，改主域后需重启后端生效。
	site.SetRootDomain(cfg.Brand.RootDomain)

	// 初始化 Redis
	if err := redis.Init(&cfg.Redis); err != nil {
		log.Fatalf("初始化 Redis 失败: %v", err)
	}
	defer redis.Close()
	log.Println("Redis 连接成功")

	// 初始化路由
	r, authService, balanceService, notifyService := router.Setup(cfg)

	// 构建支付客户端：渠道未启用（*_ENABLED=2）、凭据缺失或凭据有误时跳过该渠道，
	// 仅记录日志、不影响服务启动与其它支付渠道
	var alipayClient *upstream.AlipayClient
	if config.PaymentChannelEnabled(cfg.Alipay.Enabled) {
		client, aerr := upstream.NewAlipayClient(cfg.Alipay.AppID, cfg.Alipay.PrivateKey, cfg.Alipay.PublicKey)
		if aerr != nil {
			log.Printf("初始化支付宝客户端失败，支付宝支付不可用: %v", aerr)
		}
		alipayClient = client
	}
	var wechatPayClient *upstream.WechatPayClient
	if config.PaymentChannelEnabled(cfg.WechatPay.Enabled) {
		client, werr := upstream.NewWechatPayClient(
			cfg.WechatPay.AppID, cfg.WechatPay.MchID, cfg.WechatPay.ApiV3Key,
			cfg.WechatPay.MerchantPrivKey, cfg.WechatPay.MchSerialNo, cfg.WechatPay.PublicKey,
		)
		if werr != nil {
			log.Printf("初始化微信支付客户端失败，微信支付不可用: %v", werr)
		}
		wechatPayClient = client
	}

	// 启动定时任务
	cronManager := cron.NewCronManager(authService, balanceService, notifyService, alipayClient, wechatPayClient)
	if err := cronManager.Start(); err != nil {
		log.Fatalf("启动定时任务失败: %v", err)
	}
	defer cronManager.Stop()

	// 启动服务器
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("服务器启动在 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
