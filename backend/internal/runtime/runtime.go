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

	// 企业工商四要素核验 provider：默认腾讯云 OCR；配置为 aliyun 时由阿里云提供（未配置时不可用）
	if cfg.EnterpriseVerifyProvider != config.EnterpriseVerifyProviderTencent {
		log.Printf("企业四要素核验 provider=%s 尚未配置，企业实名自助核验不可用", cfg.EnterpriseVerifyProvider)
	} else if ocr := newTencentOcr(cfg); ocr != nil {
		s.enterpriseVerifier = upstream.NewTencentOcrVerifier(ocr)
	}

	// 人机验证码
	s.captchaAppID = cfg.Tencent.Captcha.CaptchaAppID
	s.captcha = service.NewCaptchaService(
		cfg.Tencent.SecretID,
		cfg.Tencent.SecretKey,
		s.captchaAppID,
		cfg.Tencent.Captcha.AppSecretKey,
	)

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