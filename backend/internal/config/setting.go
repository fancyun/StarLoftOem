package config

import (
	"fmt"
	"strconv"
	"strings"

	"oemrpa/internal/model"
)

// IsSecretSettingKey 判断配置键是否为密钥类（密钥/口令/私钥/AppKey）。
// 密钥类配置一律只由 .env 提供，不纳入配置表：既不入库也无法在后台维护。
// 判定规则与后台「系统设置」的脱敏规则一致（含 SECRET/PASSWORD/PRIVATE_KEY/APIKEY/API_KEY/KEY）。
func IsSecretSettingKey(key string) bool {
	k := strings.ToUpper(key)
	for _, s := range []string{"SECRET", "PASSWORD", "PRIVATE_KEY", "APIKEY", "API_KEY", "KEY"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// ApplySettingOverrides 用数据库中的配置覆盖代码内置默认值：
//   - sys：系统库 setting 表的非密钥业务配置（数据库有值则以数据库为准，缺省保留内置默认值）
//   - fv / sms：产品库 product_config 表的平台单价
//
// 密钥类配置（数据库/Redis 连接、JWT、数据加密，以及第三方密钥/AppKey）不在此覆盖范围内，
// 只由 .env 提供；此处会剔除 sys 中的密钥键，确保历史遗留行也不会被读入。
func ApplySettingOverrides(cfg *Config, sys, fv, sms map[string]string) {
	for k := range sys {
		if IsSecretSettingKey(k) {
			delete(sys, k)
		}
	}

	// FinAuth（下游实名/人脸核验）：API_KEY/API_SECRET 为密钥类，只走 .env
	applyString(&cfg.FinAuth.SceneID, sys, "FINAUTH_SCENE_ID")
	applyString(&cfg.FinAuth.BaseURL, sys, "FINAUTH_BASE_URL")

	// 腾讯云：SECRET_ID/SECRET_KEY/CAPTCHA_SECRET 为密钥类（只走 .env），标识与地域可由数据库维护
	applyString(&cfg.TencentRegion, sys, "TENCENT_REGION")
	applyString(&cfg.Tencent.Captcha.CaptchaAppID, sys, "TENCENT_CAPTCHA_APP_ID")
	applyString(&cfg.TencentSmsSdkAppID, sys, "PLATFORM_TENCENT_SMS_SDKAPPID")
	applyString(&cfg.TencentSmsVerifySign, sys, "PLATFORM_TENCENT_SMS_VERIFY_SIGN")
	applyString(&cfg.TencentSmsVerifyTemplateID, sys, "PLATFORM_TENCENT_SMS_VERIFY_TEMPLATE_ID")
	applyString(&cfg.TencentFaceIdRuleId, sys, "PLATFORM_TENCENT_FACEID_RULE_ID")

	// 支付宝支付（PEM 证书走 certs/ 目录文件，不入配置表）
	applyInt(&cfg.Alipay.Enabled, sys, "ALIPAY_ENABLED")
	applyString(&cfg.Alipay.AppID, sys, "ALIPAY_APP_ID")
	applyInt(&cfg.WechatPay.Enabled, sys, "WECHAT_ENABLED")
	applyString(&cfg.WechatPay.AppID, sys, "WECHAT_APP_ID")
	applyString(&cfg.WechatPay.MchID, sys, "WECHAT_MCH_ID")
	applyString(&cfg.WechatPay.MchSerialNo, sys, "WECHAT_MCH_SERIAL_NO")

	// 短信上游（联麓）：AppKey 为密钥类（只走 .env）
	applyString(&cfg.Shlianlu.MchID, sys, "SHLIANLU_MCH_ID")
	applyString(&cfg.Shlianlu.AppID, sys, "SHLIANLU_APP_ID")
	applyString(&cfg.Shlianlu.MarketingAppID, sys, "SHLIANLU_MARKETING_APP_ID")
	applyString(&cfg.Shlianlu.BaseURL, sys, "SHLIANLU_BASE_URL")

	// 产品库平台单价（人脸核验库仅产品自身单价）
	applyFloat(&cfg.FvAuthPrice, fv, model.ProductConfigFvAuthPrice)
	applyFloat(&cfg.FvSelfPrice, fv, model.ProductConfigFvSelfPrice)
	// 账户实名单价属平台账户能力，存系统库设置表
	applyFloat(&cfg.KycPersonalPrice, sys, SettingKeyKycPersonalPrice)
	applyFloat(&cfg.KycEnterprisePrice, sys, SettingKeyKycEnterprisePrice)
	applyFloat(&cfg.SMSPrice, sms, model.ProductConfigSmsPrice)
	// 各产品成本单价：允许为 0（无成本），故不沿用 applyFloat 的正数限制
	applyFloatZero(&cfg.FvAuthCost, fv, model.ProductConfigFvAuthCost)
	applyFloatZero(&cfg.FvSelfCost, fv, model.ProductConfigFvSelfCost)
	applyFloatZero(&cfg.SmsCost, sms, model.ProductConfigSmsCost)

	// 支付风控
	applyFloat(&cfg.PaymentDailyLimit, sys, SettingKeyPaymentDailyLimit)
	// 推广分佣：提成比例允许为 0（表示不提成），故不沿用 applyFloat 的正数限制
	applyFloatZero(&cfg.AffCommissionRateFV, sys, SettingKeyAffCommissionRateFV)
	applyFloatZero(&cfg.AffCommissionRateSMS, sys, SettingKeyAffCommissionRateSMS)

	// 客服联系方式（门户首页展示；首页经公开配置接口实时读取配置表，此处仅同步启动时的生效值）
	applyString(&cfg.Contact.Email, sys, SettingKeyContactEmail)
	applyString(&cfg.Contact.Phone, sys, SettingKeyContactPhone)
	applyString(&cfg.Contact.Wechat, sys, SettingKeyContactWechat)
	applyString(&cfg.Contact.QQ, sys, SettingKeyContactQQ)
	applyString(&cfg.Contact.Hours, sys, SettingKeyContactHours)
}

// 账户实名单价配置键（存系统库设置表）
const (
	SettingKeyKycPersonalPrice   = "KYC_PERSONAL_PRICE"
	SettingKeyKycEnterprisePrice = "KYC_ENTERPRISE_PRICE"
	// 支付风控
	SettingKeyPaymentDailyLimit = "PAYMENT_DAILY_LIMIT"
	// 推广分佣（全局统一：所有推广商一律按此计提，改动即时生效）
	SettingKeyAffCommissionRateFV  = "AFF_COMMISSION_RATE_FV"
	SettingKeyAffCommissionRateSMS = "AFF_COMMISSION_RATE_SMS"
	// 客服联系方式（门户首页每次加载实时读取，改动即时生效；留空表示首页不展示该项）
	SettingKeyContactEmail  = "SERVICE_CONTACT_EMAIL"
	SettingKeyContactPhone  = "SERVICE_CONTACT_PHONE"
	SettingKeyContactWechat = "SERVICE_CONTACT_WECHAT"
	SettingKeyContactQQ     = "SERVICE_CONTACT_QQ"
	SettingKeyContactHours  = "SERVICE_CONTACT_HOURS"
)

// SettingSpec 系统配置目录项：键名、分组与说明
type SettingSpec struct {
	Key      string
	Category string
	Remark   string
}

// SettingCatalog 全部可由数据库管理的系统配置键（后台「系统设置」按此目录预置，用户改值即可）。
// 仅收录非密钥类配置：数据库连接/Redis/JWT/数据加密等自举密钥，以及第三方密钥/AppKey（判定见
// IsSecretSettingKey）均不在此列，只由 .env 提供。
func SettingCatalog() []SettingSpec {
	return []SettingSpec{
		// 实名核验（FinAuth）：API Key/Secret 为密钥类，只走 .env
		{"FINAUTH_SCENE_ID", model.SettingCategoryFinAuth, "FinAuth 场景 ID"},
		{"FINAUTH_BASE_URL", model.SettingCategoryFinAuth, "FinAuth 接口基址"},

		// 腾讯云（验证码 / 平台短信 / 人脸核身 / OCR）：SecretId/SecretKey/验证码 AppSecretKey 为密钥类，只走 .env
		{"TENCENT_REGION", model.SettingCategoryTencent, "腾讯云地域（如 ap-guangzhou）"},
		{"TENCENT_CAPTCHA_APP_ID", model.SettingCategoryTencent, "天御验证码 AppID"},
		{"PLATFORM_TENCENT_SMS_SDKAPPID", model.SettingCategoryTencent, "平台验证码短信 SDKAppID"},
		{"PLATFORM_TENCENT_SMS_VERIFY_SIGN", model.SettingCategoryTencent, "平台验证码短信签名（已审核）"},
		{"PLATFORM_TENCENT_SMS_VERIFY_TEMPLATE_ID", model.SettingCategoryTencent, "平台验证码短信模板 ID（已审核）"},
		{"PLATFORM_TENCENT_FACEID_RULE_ID", model.SettingCategoryTencent, "法人扫脸人脸核身 RuleId"},

		// 支付宝支付（APP_ID 为标识；PEM 证书见 certs/ 目录，均不纳入配置表）
		{"ALIPAY_ENABLED", model.SettingCategoryAlipay, "启用开关：1-启用 2-不启用"},
		{"ALIPAY_APP_ID", model.SettingCategoryAlipay, "支付宝应用 AppID"},

		// 微信支付（API v3）：APIv3 密钥为密钥类，只走 .env；PEM 证书见 certs/ 目录
		{"WECHAT_ENABLED", model.SettingCategoryWechat, "启用开关：1-启用 2-不启用"},
		{"WECHAT_APP_ID", model.SettingCategoryWechat, "微信 AppID"},
		{"WECHAT_MCH_ID", model.SettingCategoryWechat, "微信商户号"},
		{"WECHAT_MCH_SERIAL_NO", model.SettingCategoryWechat, "商户 API 证书序列号"},

		// 短信服务（联麓，下游短信产品唯一上游）：AppKey 为密钥类，只走 .env
		{"SHLIANLU_MCH_ID", model.SettingCategorySMS, "联麓企业 ID（MchId）"},
		{"SHLIANLU_APP_ID", model.SettingCategorySMS, "联麓应用 ID（AppId，验证码/通知短信通道）"},
		{"SHLIANLU_MARKETING_APP_ID", model.SettingCategorySMS, "联麓应用 ID（AppId，营销短信通道；留空表示营销短信不可用）"},
		{"SHLIANLU_BASE_URL", model.SettingCategorySMS, "联麓 API 基址（如 https://apis.shlianlu.com）"},

		// 账户实名单价（平台账户能力，成本由平台承担；人脸核验产品单价见各产品分区「产品配置」）
		{SettingKeyKycPersonalPrice, model.SettingCategoryKYC, "个人实名免费次数用尽后单价（元/次）"},
		{SettingKeyKycEnterprisePrice, model.SettingCategoryKYC, "企业实名免费次数用尽后单价（元/次）"},

		// 支付风控
		{SettingKeyPaymentDailyLimit, model.SettingCategoryPayment, "单用户单日在线支付金额上限（元，留空或 0 表示不限）"},

		// 推广分佣（全局统一：所有推广商与员工销售一律按此计提，改完即时生效；计提基数为利润）
		{SettingKeyAffCommissionRateFV, model.SettingCategoryAff, "人脸核验提成比例（0~1，如 0.2 表示按下级利润的 20% 计提；0 表示不提成）。利润 = 实付 − 成本 × 次数，成本在「人脸核验 → 产品配置」设置；全局统一，即时生效"},
		{SettingKeyAffCommissionRateSMS, model.SettingCategoryAff, "短信提成比例（0~1，如 0.2 表示按下级利润的 20% 计提；0 表示不提成）。利润 = 实付 − 成本 × 条数，成本在「短信服务 → 产品配置」设置；全局统一，即时生效"},

		// 客服联系方式（门户首页页脚展示，门户每次加载实时读取，改完即时生效；留空表示首页不展示该项）
		{SettingKeyContactEmail, model.SettingCategoryContact, "客服邮箱（如 service@example.com）"},
		{SettingKeyContactPhone, model.SettingCategoryContact, "客服电话（如 400-000-0000）"},
		{SettingKeyContactWechat, model.SettingCategoryContact, "客服微信号"},
		{SettingKeyContactQQ, model.SettingCategoryContact, "客服 QQ 号"},
		{SettingKeyContactHours, model.SettingCategoryContact, "服务时间（如 周一至周日 8:00 - 24:00）"},
	}
}

// SettingValues 取当前生效配置的键值（供启动时预置配置表：表内缺失的键按此写入，已有值不覆盖）。
// 仅返回非密钥类配置；密钥类只由 .env 提供，不入配置表。
func (cfg *Config) SettingValues() map[string]string {
	return map[string]string{
		"FINAUTH_SCENE_ID":                        cfg.FinAuth.SceneID,
		"FINAUTH_BASE_URL":                        cfg.FinAuth.BaseURL,
		"TENCENT_REGION":                          cfg.TencentRegion,
		"TENCENT_CAPTCHA_APP_ID":                  cfg.Tencent.Captcha.CaptchaAppID,
		"PLATFORM_TENCENT_SMS_SDKAPPID":           cfg.TencentSmsSdkAppID,
		"PLATFORM_TENCENT_SMS_VERIFY_SIGN":        cfg.TencentSmsVerifySign,
		"PLATFORM_TENCENT_SMS_VERIFY_TEMPLATE_ID": cfg.TencentSmsVerifyTemplateID,
		"PLATFORM_TENCENT_FACEID_RULE_ID":         cfg.TencentFaceIdRuleId,
		"ALIPAY_ENABLED":                          strconv.Itoa(cfg.Alipay.Enabled),
		"ALIPAY_APP_ID":                           cfg.Alipay.AppID,
		"WECHAT_ENABLED":                          strconv.Itoa(cfg.WechatPay.Enabled),
		"WECHAT_APP_ID":                           cfg.WechatPay.AppID,
		"WECHAT_MCH_ID":                           cfg.WechatPay.MchID,
		"WECHAT_MCH_SERIAL_NO":                    cfg.WechatPay.MchSerialNo,
		"SHLIANLU_MCH_ID":                         cfg.Shlianlu.MchID,
		"SHLIANLU_APP_ID":                         cfg.Shlianlu.AppID,
		"SHLIANLU_MARKETING_APP_ID":               cfg.Shlianlu.MarketingAppID,
		"SHLIANLU_BASE_URL":                       cfg.Shlianlu.BaseURL,
		SettingKeyKycPersonalPrice:                formatFloat(cfg.KycPersonalPrice),
		SettingKeyKycEnterprisePrice:              formatFloat(cfg.KycEnterprisePrice),
		SettingKeyPaymentDailyLimit:               formatFloat(cfg.PaymentDailyLimit),
		SettingKeyAffCommissionRateFV:             formatFloat(cfg.AffCommissionRateFV),
		SettingKeyAffCommissionRateSMS:            formatFloat(cfg.AffCommissionRateSMS),
		SettingKeyContactEmail:                    cfg.Contact.Email,
		SettingKeyContactPhone:                    cfg.Contact.Phone,
		SettingKeyContactWechat:                   cfg.Contact.Wechat,
		SettingKeyContactQQ:                       cfg.Contact.QQ,
		SettingKeyContactHours:                    cfg.Contact.Hours,
	}
}

// ProductConfigSpec 产品配置目录项
type ProductConfigSpec struct {
	Key    string
	Remark string
}

// FvProductConfigCatalog 人脸核验产品配置目录（后台「人脸核验 → 产品配置」按此预置）
func FvProductConfigCatalog() []ProductConfigSpec {
	return []ProductConfigSpec{
		{model.ProductConfigFvAuthPrice, "有源人脸核验单价（元/次）"},
		{model.ProductConfigFvSelfPrice, "无源人脸核验单价（元/次）"},
		{model.ProductConfigFvAuthCost, "有源人脸核验成本单价（元/次）：不计入售价，仅用于按利润计提推广提成（利润 = 实付 − 成本 × 次数），留空或 0 视为无成本"},
		{model.ProductConfigFvSelfCost, "无源人脸核验成本单价（元/次）：不计入售价，仅用于按利润计提推广提成，留空或 0 视为无成本"},
	}
}

// FvProductConfigValues 人脸核验产品配置当前生效值
func (cfg *Config) FvProductConfigValues() map[string]string {
	return map[string]string{
		model.ProductConfigFvAuthPrice: formatFloat(cfg.FvAuthPrice),
		model.ProductConfigFvSelfPrice: formatFloat(cfg.FvSelfPrice),
		model.ProductConfigFvAuthCost:  formatFloat(cfg.FvAuthCost),
		model.ProductConfigFvSelfCost:  formatFloat(cfg.FvSelfCost),
	}
}

// SmsProductConfigCatalog 短信产品配置目录
func SmsProductConfigCatalog() []ProductConfigSpec {
	return []ProductConfigSpec{
		{model.ProductConfigSmsPrice, "平台短信单价（元/条），未配置时代码层兜底 0.05"},
		{model.ProductConfigSmsCost, "短信成本单价（元/条）：不计入售价，仅用于按利润计提推广提成（利润 = 实付 − 成本 × 条数），留空或 0 视为无成本"},
	}
}

// SmsProductConfigValues 短信产品配置当前生效值
func (cfg *Config) SmsProductConfigValues() map[string]string {
	return map[string]string{
		model.ProductConfigSmsPrice: formatFloat(cfg.SMSPrice),
		model.ProductConfigSmsCost:  formatFloat(cfg.SmsCost),
	}
}

// formatFloat 数值配置格式化（0 表示未配置，预置时写空串以便后台提示补填）
func formatFloat(v float64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// MissingThirdPartyKeys 返回第三方业务配置中缺失的关键项（不影响启动，仅提示对应能力不可用）。
// 自举必需的数据库/Redis/JWT/加密密钥不在此列（由 config.validate 在启动早期强校验）。
func (cfg *Config) MissingThirdPartyKeys() []string {
	checks := []struct {
		name string
		val  string
	}{
		{"TENCENT_SECRET_ID", cfg.Tencent.SecretID},
		{"TENCENT_SECRET_KEY", cfg.Tencent.SecretKey},
		{"TENCENT_CAPTCHA_APP_ID", cfg.Tencent.Captcha.CaptchaAppID},
		{"TENCENT_CAPTCHA_SECRET", cfg.Tencent.Captcha.AppSecretKey},
	}
	var missing []string
	for _, c := range checks {
		if c.val == "" {
			missing = append(missing, c.name)
		}
	}
	return missing
}

// applyString 用 kv 中非空的键值覆盖目标字符串
func applyString(dst *string, kv map[string]string, key string) {
	if v, ok := kv[key]; ok && v != "" {
		*dst = v
	}
}

// applyFloat 用 kv 中可解析为正数的键值覆盖目标浮点数（PEM 等多行值不适用）
func applyFloat(dst *float64, kv map[string]string, key string) {
	v, ok := kv[key]
	if !ok || v == "" {
		return
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
		*dst = n
	}
}

// applyFloatZero 用 kv 中可解析为「非负」的键值覆盖目标浮点数（允许 0，如提成比例 0 表示不提成）
func applyFloatZero(dst *float64, kv map[string]string, key string) {
	v, ok := kv[key]
	if !ok || v == "" {
		return
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 {
		*dst = n
	}
}

// applyInt 用 kv 中可解析的键值覆盖目标整数
func applyInt(dst *int, kv map[string]string, key string) {
	v, ok := kv[key]
	if !ok || v == "" {
		return
	}
	if n, err := strconv.Atoi(v); err == nil {
		*dst = n
	}
}

// FormatMissingKeys 将缺失键列表格式化为日志文本
func FormatMissingKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return fmt.Sprintf("第三方配置缺失（对应能力不可用）：%v", keys)
}
