package runtime

import (
	"log"
	"sync"

	"oemrpa/internal/config"
	"oemrpa/internal/service"
	"oemrpa/internal/upstream"
)

// Runtime 持有第三方业务配置对应的上游客户端快照。
// 业务配置全部来自环境变量（config.Load 加载），启动时一次性构建，不依赖数据库配置。
type Runtime struct {
	mu  sync.RWMutex
	snp *snapshot
}

// snapshot 当前生效的第三方业务客户端快照。
type snapshot struct {
	finAuth      upstream.FinAuthInterface
	finAuthCfg   config.FinAuthConfig
	alipay       *upstream.AlipayClient
	wechatPay    *upstream.WechatPayClient
	sms          *service.SMSService
	captcha      *service.CaptchaService
	captchaAppID string
	smsPrice     float64
	fvAuthPrice  float64
	fvSelfPrice  float64
	// 各产品成本单价（元/次、元/条）：仅用于按利润计提推广提成，不参与售价
	fvAuthCost float64
	fvSelfCost float64
	smsCost    float64
	// 下游短信产品唯一上游（联麓）
	shlianlu *upstream.ShlianluClient
	// 平台自用腾讯云：法人扫脸（人脸核身）客户端
	platformFaceId *upstream.TencentFaceIdClient
	// 平台自用腾讯云：营业执照核验（企业四要素）客户端
	platformOcr *upstream.TencentOcrClient
	// 平台自用腾讯云：验证码短信通道
	platformSms *upstream.TencentSmsClient
}

// New 根据环境变量配置构建第三方业务客户端快照。
func New(cfg *config.Config) (*Runtime, error) {
	rt := &Runtime{}
	s := &snapshot{}

	// FinAuth（下游实名/人脸核验/账户实名个人）
	s.finAuthCfg = cfg.FinAuth
	s.finAuth = upstream.NewFinAuthClient(s.finAuthCfg.BaseURL, s.finAuthCfg.APIKey, s.finAuthCfg.APISecret)

	// 腾讯云账号密钥（验证码、平台自用短信/人脸核身共用）
	secretID := cfg.Tencent.SecretID
	secretKey := cfg.Tencent.SecretKey
	region := cfg.TencentRegion

	// 下游短信产品唯一上游（联麓 shlianlu，配置齐全时构建，否则 nil 由调用方兜底）；
	// 营销短信应用（MarketingAppID）可缺省，缺省时营销模板报备与营销短信发送不可用
	if cfg.Shlianlu.MchID != "" && cfg.Shlianlu.AppID != "" && cfg.Shlianlu.Key != "" && cfg.Shlianlu.BaseURL != "" {
		s.shlianlu = upstream.NewShlianluClient(cfg.Shlianlu.BaseURL, cfg.Shlianlu.MchID, cfg.Shlianlu.AppID, cfg.Shlianlu.MarketingAppID, cfg.Shlianlu.Key)
	}

	// 平台自用腾讯云：企业实名法人扫脸（人脸核身 NeedAuth/RuleId 已配置时构建）
	if cfg.TencentFaceIdRuleId != "" && secretID != "" && secretKey != "" {
		s.platformFaceId = upstream.NewTencentFaceIdClient(secretID, secretKey, region, cfg.TencentFaceIdRuleId)
	}

	// 平台自用腾讯云：营业执照核验（企业四要素），腾讯云账号密钥已配置时构建
	if secretID != "" && secretKey != "" {
		s.platformOcr = upstream.NewTencentOcrClient(secretID, secretKey, region)
	}

	// 平台自用腾讯云：验证码短信通道（SDKAppID 已配置时构建）
	if cfg.TencentSmsSdkAppID != "" && secretID != "" && secretKey != "" {
		s.platformSms = upstream.NewTencentSmsClient(secretID, secretKey, region, cfg.TencentSmsSdkAppID)
	}

	// 人机验证码
	s.captchaAppID = cfg.Tencent.Captcha.CaptchaAppID
	s.captcha = service.NewCaptchaService(
		secretID,
		secretKey,
		s.captchaAppID,
		cfg.Tencent.Captcha.AppSecretKey,
	)

	// 短信（平台验证码走腾讯云通道；未配置通道或签名时返回 nil 不启用）
	s.sms = service.NewSMSService(s.platformSms, cfg.TencentSmsVerifySign, cfg.TencentSmsVerifyTemplateID)

	// 支付宝
	if config.PaymentChannelEnabled(cfg.Alipay.Enabled) &&
		cfg.Alipay.AppID != "" && cfg.Alipay.PrivateKey != "" && cfg.Alipay.PublicKey != "" {
		alipayClient, e := upstream.NewAlipayClient(cfg.Alipay.AppID, cfg.Alipay.PrivateKey, cfg.Alipay.PublicKey)
		if e != nil {
			log.Printf("构建支付宝支付客户端失败，支付宝充值暂时不可用: %v", e)
		} else {
			s.alipay = alipayClient
		}
	}

	// 微信支付（API v3，Native 扫码 + H5 网页支付）
	if config.PaymentChannelEnabled(cfg.WechatPay.Enabled) &&
		cfg.WechatPay.AppID != "" && cfg.WechatPay.MchID != "" && cfg.WechatPay.ApiV3Key != "" &&
		cfg.WechatPay.MerchantPrivKey != "" && cfg.WechatPay.MchSerialNo != "" && cfg.WechatPay.PublicKey != "" {
		wechatPayClient, e := upstream.NewWechatPayClient(
			cfg.WechatPay.AppID, cfg.WechatPay.MchID, cfg.WechatPay.ApiV3Key,
			cfg.WechatPay.MerchantPrivKey, cfg.WechatPay.MchSerialNo, cfg.WechatPay.PublicKey,
		)
		if e != nil {
			log.Printf("构建微信支付客户端失败，微信支付暂时不可用: %v", e)
		} else {
			s.wechatPay = wechatPayClient
		}
	}

	// 平台短信单价（元/条）
	s.smsPrice = cfg.SMSPrice

	// 下游人脸核验单价（有源 fv_auth / 无源 fv_self，元/次）
	s.fvAuthPrice = cfg.FvAuthPrice
	s.fvSelfPrice = cfg.FvSelfPrice

	// 各产品成本单价（元/次、元/条）：仅用于按利润计提推广提成
	s.fvAuthCost = cfg.FvAuthCost
	s.fvSelfCost = cfg.FvSelfCost
	s.smsCost = cfg.SmsCost

	rt.snp = s
	return rt, nil
}

// FinAuth 返回当前生效的 FinAuth 客户端。
func (rt *Runtime) FinAuth() upstream.FinAuthInterface {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.finAuth
}

// FinAuthCfg 返回当前生效的 FinAuth 配置。
func (rt *Runtime) FinAuthCfg() config.FinAuthConfig {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.finAuthCfg
}

// PlatformFaceId 返回平台自用腾讯云人脸核身客户端（RuleId 未配置时为 nil）。
func (rt *Runtime) PlatformFaceId() *upstream.TencentFaceIdClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.platformFaceId
}

// PlatformOcr 返回平台自用腾讯云 OCR 客户端（密钥未配置时为 nil）。
func (rt *Runtime) PlatformOcr() *upstream.TencentOcrClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.platformOcr
}

// Alipay 返回当前生效的支付宝客户端（未配置时为 nil）。
func (rt *Runtime) Alipay() *upstream.AlipayClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.alipay
}

// WechatPay 返回当前生效的微信支付客户端（未配置时为 nil）。
func (rt *Runtime) WechatPay() *upstream.WechatPayClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.wechatPay
}

// SMS 返回当前生效的短信服务（平台验证码）。
func (rt *Runtime) SMS() *service.SMSService {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.sms
}

// Captcha 返回当前生效的人机验证码服务。
func (rt *Runtime) Captcha() *service.CaptchaService {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.captcha
}

// CaptchaAppID 返回当前生效的验证码 AppID（供前端渲染验证码组件）。
func (rt *Runtime) CaptchaAppID() string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.captchaAppID
}

// SMSPrice 返回平台短信单价（元/条）。
func (rt *Runtime) SMSPrice() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.smsPrice
}

// FvAuthPrice 返回下游有源人脸核验单价（元/次，未配置时兜底 1.00）。
func (rt *Runtime) FvAuthPrice() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if rt.snp.fvAuthPrice > 0 {
		return rt.snp.fvAuthPrice
	}
	return 1.00
}

// FvSelfPrice 返回下游无源人脸核验单价（元/次，未配置时兜底 1.00）。
func (rt *Runtime) FvSelfPrice() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if rt.snp.fvSelfPrice > 0 {
		return rt.snp.fvSelfPrice
	}
	return 1.00
}

// FvAuthCost 返回有源人脸核验成本单价（元/次，未配置为 0=无成本）：仅用于按利润计提推广提成。
func (rt *Runtime) FvAuthCost() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.fvAuthCost
}

// FvSelfCost 返回无源人脸核验成本单价（元/次，未配置为 0=无成本）：仅用于按利润计提推广提成。
func (rt *Runtime) FvSelfCost() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.fvSelfCost
}

// SmsCost 返回短信成本单价（元/条，未配置为 0=无成本）：仅用于按利润计提推广提成。
func (rt *Runtime) SmsCost() float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.smsCost
}

// Shlianlu 返回下游短信产品唯一上游（联麓）客户端（未配置时为 nil，调用方按 nil 兜底）。
func (rt *Runtime) Shlianlu() *upstream.ShlianluClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.shlianlu
}
