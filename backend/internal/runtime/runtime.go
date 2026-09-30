package runtime

import (
	"log"
	"sync"

	"oemrpa/internal/config"
	"oemrpa/internal/service"
	"oemrpa/internal/upstream"
	"oemrpa/internal/upstream/starloft"
)

// Runtime 持有第三方业务配置对应的上游客户端快照。
// 业务配置全部来自环境变量与数据库配置（见 config.Load / config.ApplySettingOverrides），启动时一次性构建。
type Runtime struct {
	mu  sync.RWMutex
	snp *snapshot
}

// snapshot 当前生效的第三方业务客户端快照。
type snapshot struct {
	// starLoft 上游 StarLoft 平台客户端：OEM 系统的唯一上游（短信产品 / 人脸核验产品 / 账户实名扫脸）
	starLoft *starloft.Client
	// finAuth 人脸核验（FV）产品上游（StarLoft 平台开放 API）
	finAuth upstream.FinAuthInterface
	// smsUpstream 短信产品上游（StarLoft 平台开放 API）
	smsUpstream service.SmsUpstream
	// faceProvider 账户实名人脸核身 provider（上游平台 / 腾讯云可切换）
	faceProvider upstream.FaceProvider
	// enterpriseVerifier 企业工商四要素核验 provider（腾讯云 OCR / 阿里云可切换）
	enterpriseVerifier upstream.EnterpriseVerifier
	// smsPushVerifier 上游短信推送（回执/回复/签名与模板状态）签名校验器
	smsPushVerifier upstream.SmsPushVerifier

	alipay    *upstream.AlipayClient
	wechatPay *upstream.WechatPayClient
	// wechatOAuth 微信一键登录（公众号网页授权：手机端页面内授权 / PC 扫码授权）
	wechatOAuth *upstream.WechatOAuthClient
	sms         *service.SMSService
	// captchaProvider 人机验证码通道（天御 / 极验 / 阿里云，按后台配置切换）
	captchaProvider service.CaptchaProvider
	smsPrice        float64
	fvAuthPrice  float64
	fvSelfPrice  float64
	// 各产品成本单价（元/次、元/条）：仅用于按利润计提推广提成，不参与售价
	fvAuthCost float64
	fvSelfCost float64
	smsCost    float64
}

// New 根据配置构建第三方业务客户端快照。
func New(cfg *config.Config) (*Runtime, error) {
	rt := &Runtime{}
	s := &snapshot{}

	// 上游 StarLoft 平台（唯一上游）：短信产品、人脸核验产品、账户实名扫脸共用同一客户端
	s.starLoft = starloft.New(cfg.StarLoft.BaseURL, cfg.StarLoft.APIKey, cfg.StarLoft.APISecret, cfg.StarLoft.MarketingEnabled)
	if !s.starLoft.Available() {
		log.Print("上游 StarLoft 平台未配置（STARLOFT_API_KEY / STARLOFT_API_SECRET / STARLOFT_API_BASE_URL），短信、人脸核验与账户实名能力不可用")
	}
	s.finAuth = s.starLoft
	s.smsUpstream = s.starLoft
	s.smsPushVerifier = s.starLoft

	// 账户实名人脸核身 provider：默认走上游 StarLoft 平台；配置为 tencent 时回落腾讯云人脸核身
	if cfg.FaceProvider == config.FaceProviderTencent {
		if faceId := newTencentFaceId(cfg); faceId != nil {
			s.faceProvider = upstream.NewTencentFaceProvider(faceId)
		} else {
			log.Print("账户实名人脸核身 provider 配置为腾讯云，但 RuleId 或腾讯云密钥缺失，人脸核身不可用")
		}
	} else {
		s.faceProvider = s.starLoft
	}

	// 企业工商四要素核验 provider：默认腾讯云 OCR；配置为 aliyun 时走阿里云云市场（凭据缺失时不可用）
	if cfg.EnterpriseVerifyProvider == config.EnterpriseVerifyProviderTencent {
		if ocr := newTencentOcr(cfg); ocr != nil {
			s.enterpriseVerifier = upstream.NewTencentOcrVerifier(ocr)
		}
	} else {
		s.enterpriseVerifier = upstream.NewAliyunEnterpriseVerifier(cfg.AliyunEnterpriseVerifyURL, cfg.AliyunMarketAppCode)
		if cfg.AliyunEnterpriseVerifyURL == "" || cfg.AliyunMarketAppCode == "" {
			log.Print("企业四要素核验 provider=aliyun，但接口地址或 AppCode 缺失，企业实名自助核验不可用")
		}
	}

	// 人机验证码：按后台配置选择通道（天御 / 极验 / 阿里云）
	tencentCaptcha := service.NewCaptchaService(
		cfg.Tencent.SecretID,
		cfg.Tencent.SecretKey,
		cfg.Tencent.Captcha.CaptchaAppID,
		cfg.Tencent.Captcha.AppSecretKey,
	)
	switch cfg.CaptchaProvider {
	case config.CaptchaProviderGeetest:
		s.captchaProvider = service.NewGeetestCaptchaProvider(cfg.GeetestCaptchaID, cfg.GeetestCaptchaKey)
	case config.CaptchaProviderAliyun:
		s.captchaProvider = service.NewAliyunCaptchaProvider(cfg.AliyunAccessKeyID, cfg.AliyunAccessKeySecret, cfg.AliyunCaptchaSceneID)
	default:
		s.captchaProvider = service.NewTencentCaptchaProvider(tencentCaptcha, cfg.Tencent.Captcha.CaptchaAppID)
	}

	// 平台验证码短信：经上游 StarLoft 平台下发（签名与模板须为上游已审核通过）
	s.sms = service.NewSMSService(s.starLoft, cfg.PlatformSmsSign, cfg.PlatformSmsTemplateID)

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

	// 微信一键登录（公众号网页授权）：开关启用且已配置公众号 AppID 时构建；
	// 凭据是否齐备由客户端 Available() 判定
	if config.PaymentChannelEnabled(cfg.WechatLogin.Enabled) && cfg.WechatLogin.MPAppID != "" {
		wechatOAuthClient, e := upstream.NewWechatOAuthClient(
			cfg.WechatLogin.MPAppID, cfg.WechatLogin.MPAppSecret,
		)
		if e != nil {
			log.Printf("构建微信登录客户端失败，微信一键登录暂时不可用: %v", e)
		} else {
			s.wechatOAuth = wechatOAuthClient
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

// newTencentFaceId 构建腾讯云人脸核身客户端（RuleId 与账号密钥齐备时）
func newTencentFaceId(cfg *config.Config) *upstream.TencentFaceIdClient {
	if cfg.TencentFaceIdRuleId == "" || cfg.Tencent.SecretID == "" || cfg.Tencent.SecretKey == "" {
		return nil
	}
	return upstream.NewTencentFaceIdClient(cfg.Tencent.SecretID, cfg.Tencent.SecretKey, cfg.TencentRegion, cfg.TencentFaceIdRuleId)
}

// newTencentOcr 构建腾讯云 OCR 客户端（账号密钥齐备时）
func newTencentOcr(cfg *config.Config) *upstream.TencentOcrClient {
	if cfg.Tencent.SecretID == "" || cfg.Tencent.SecretKey == "" {
		return nil
	}
	return upstream.NewTencentOcrClient(cfg.Tencent.SecretID, cfg.Tencent.SecretKey, cfg.TencentRegion)
}

// FinAuth 返回人脸核验（FV）产品上游客户端。
func (rt *Runtime) FinAuth() upstream.FinAuthInterface {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.finAuth
}

// SmsUpstream 返回短信产品上游客户端。
func (rt *Runtime) SmsUpstream() service.SmsUpstream {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.smsUpstream
}

// FaceProvider 返回账户实名人脸核身 provider（未配置时为 nil）。
func (rt *Runtime) FaceProvider() upstream.FaceProvider {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.faceProvider
}

// EnterpriseVerifier 返回企业工商四要素核验 provider（未配置时为 nil）。
func (rt *Runtime) EnterpriseVerifier() upstream.EnterpriseVerifier {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.enterpriseVerifier
}

// UpstreamFvBase 返回上游平台人脸核验承接页基址（{service 站点}/service/fv）：
// 承接页据此拼接 {基址}/{auth|self}?token=... 跳转到平台完成核身；上游未配置时返回空串。
func (rt *Runtime) UpstreamFvBase() string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if rt.snp.starLoft == nil {
		return ""
	}
	base := rt.snp.starLoft.ServiceBase()
	if base == "" {
		return ""
	}
	return base + "/service/fv"
}

// SmsPushVerifier 返回上游短信推送签名校验器（未配置时为 nil）。
func (rt *Runtime) SmsPushVerifier() upstream.SmsPushVerifier {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.smsPushVerifier
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

// WechatOAuth 返回当前生效的微信登录客户端（未配置时为 nil）。
func (rt *Runtime) WechatOAuth() *upstream.WechatOAuthClient {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.wechatOAuth
}

// SMS 返回当前生效的短信服务（平台验证码）。
func (rt *Runtime) SMS() *service.SMSService {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.sms
}

// CaptchaProvider 返回当前生效的人机验证码通道 provider。
func (rt *Runtime) CaptchaProvider() service.CaptchaProvider {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.snp.captchaProvider
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