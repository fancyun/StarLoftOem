package model

import "time"

// Setting 系统配置（非密钥业务配置，存于系统库）
// 说明：数据库连接/Redis 等自举参数与第三方密钥由 .env 提供；本表存可在后台在线调整的
// 非密钥业务配置，读取时以本表为准、代码内置默认值仅在表内无值时兜底。
func (Setting) TableName() string { return SysDB + ".setting" }

// 配置分组
const (
	SettingCategoryStarLoft = "starloft" // 上游 StarLoft 平台（唯一上游：短信/人脸核验/系统验证码短信/账户实名）
	SettingCategoryTencent  = "tencent"  // 腾讯云账号密钥（验证码/人脸核身/OCR）
	SettingCategoryAlipay   = "alipay"   // 支付宝支付
	SettingCategoryWechat   = "wechat"   // 微信支付
	SettingCategorySMS      = "sms"      // 短信业务（平台验证码短信签名/模板）
	SettingCategoryKYC      = "kyc"      // 账户实名单价（平台账户能力，由平台承担成本）
	SettingCategoryPayment  = "payment"  // 支付风控（在线支付单日限额等）
	SettingCategoryAff      = "aff"      // 推广分佣（提成规则）
	SettingCategoryContact  = "contact"  // 客服联系方式（门户首页展示）
	SettingCategorySecurity = "security" // JWT / 数据加密等安全密钥
	SettingCategoryCommon   = "common"   // 其它
)

type Setting struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `json:"config_key" gorm:"size:64;not null;uniqueIndex"`
	ConfigValue string    `json:"config_value" gorm:"type:text"`
	Category    string    `json:"category" gorm:"size:32;not null;default:'common';index"`
	Remark      string    `json:"remark" gorm:"size:255;not null;default:''"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// FvProductConfig 人脸核验产品配置（存于人脸核验库）
func (FvProductConfig) TableName() string { return FvDB + ".product_config" }

type FvProductConfig struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `json:"config_key" gorm:"size:64;not null;uniqueIndex"`
	ConfigValue string    `json:"config_value" gorm:"size:512;not null;default:''"`
	Remark      string    `json:"remark" gorm:"size:255;not null;default:''"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// 人脸核验产品配置键（后台「人脸核验 → 产品配置」按此预置，见 config.FvProductConfigCatalog）
// 账户实名相关单价属平台账户能力，存系统库设置表（分组 kyc），不放产品库。
const (
	ProductConfigFvAuthPrice = "fv_auth_price" // 有源人脸核验单价
	ProductConfigFvSelfPrice = "fv_self_price" // 无源人脸核验单价
	// 成本单价（元/次）：不计入售价，仅用于「利润 = 本次实付 − 成本 × 次数」并按利润计提推广提成
	ProductConfigFvAuthCost = "fv_auth_cost"
	ProductConfigFvSelfCost = "fv_self_cost"
)

// SmsProductConfig 短信产品配置（存于短信库）
func (SmsProductConfig) TableName() string { return SmsDB + ".product_config" }

type SmsProductConfig struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ConfigKey   string    `json:"config_key" gorm:"size:64;not null;uniqueIndex"`
	ConfigValue string    `json:"config_value" gorm:"size:512;not null;default:''"`
	Remark      string    `json:"remark" gorm:"size:255;not null;default:''"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// 短信产品配置键
const (
	ProductConfigSmsPrice = "sms_price" // 平台短信单价（元/条）
	// 成本单价（元/条）：不计入售价，仅用于按利润计提推广提成
	ProductConfigSmsCost = "sms_cost"
)
