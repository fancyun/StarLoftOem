package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"oemrpa/internal/model"
	"oemrpa/internal/site"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	// StarLoft 平台开放 API：OEM 系统的唯一上游（短信产品、人脸核验产品、系统验证码短信、账户实名扫脸）
	StarLoft  StarLoftConfig
	Tencent   TencentConfig
	Alipay    AlipayConfig
	WechatPay WechatPayConfig
	// 微信一键登录（公众号网页授权 / 开放平台网站应用扫码）
	WechatLogin WechatLoginConfig
	Log         LogConfig
	SMSPrice    float64 // 平台短信单价（元/条），短信计费使用（存短信产品库 product_config）
	FvAuthPrice float64 // 下游有源人脸核验单价（元/次，存人脸核验产品库 product_config，未配置时按内置默认值兜底）
	FvSelfPrice float64 // 下游无源人脸核验单价（元/次，存人脸核验产品库 product_config，未配置时按内置默认值兜底）
	// 各产品成本单价（元/次、元/条，存各产品库 product_config）：
	// 不计入售价，仅用于「利润 = 本次实付 − 成本 × 件数」并按利润计提推广提成；未配置视为 0 成本。
	FvAuthCost float64
	FvSelfCost float64
	SmsCost    float64
	// 账户实名（Web）免费次数用尽后的单价（元/次，存系统库 setting）
	KycPersonalPrice   float64
	KycEnterprisePrice float64
	// 平台自用能力：系统验证码短信（经上游 StarLoft 平台下发）与企业实名法人扫脸（腾讯云人脸核身，备选通道）
	PlatformSmsSign       string // 平台验证码短信签名（上游已审核）
	PlatformSmsTemplateID string // 平台验证码短信模板 ID（上游已审核）
	TencentFaceIdRuleId   string // 平台法人扫脸人脸核身业务流程 RuleId（腾讯云备选通道）
	// 账户实名人脸核身 provider：starloft-上游 StarLoft 平台（默认）/ tencent-腾讯云人脸核身
	FaceProvider string
	// 企业工商四要素核验 provider：tencent-腾讯云 OCR（默认）/ aliyun-阿里云
	EnterpriseVerifyProvider string
	// 人机验证码通道 provider：tencent-腾讯天御（默认）/ geetest-极验 / aliyun-阿里云行为验证码
	CaptchaProvider string
	// 极验 CaptchaId（密钥类 CaptchaKey 见 .env）
	GeetestCaptchaID string
	// 阿里云验证码场景 ID（账号密钥见 .env）
	AliyunCaptchaSceneID string
	// 极验密钥（密钥类，只走 .env）
	GeetestCaptchaKey string
	// 阿里云账号密钥与云市场 AppCode（密钥类，只走 .env）
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
	AliyunMarketAppCode   string
	// 阿里云云市场「企业工商四要素核验」接口地址（订阅商品后获得，存系统库 setting）
	AliyunEnterpriseVerifyURL string
	TencentRegion              string   // 腾讯云服务地域（如 ap-guangzhou）
	UploadDir                  string   // 用户上传文件目录（营业执照/身份证等图片）
	MediaDir                   string   // 人脸核验认证媒体目录（照片/视频，容器内 /app/media，bind mount 宿主 ./data/media）
	CertsDir                   string   // 证书目录（容器内 /app/certs，bind mount 宿主 ./certs），PEM 证书以文件形式存放于此
	DataEncryptKey             string   // 数据加密密钥（AES-256-GCM，32 字节），用于敏感字段存储加密
	PaymentExpireMinutes       int      // 待支付订单过期分钟数（PAYMENT_EXPIRE_MINUTES，默认 30），超时自动撤回支付
	PaymentDailyLimit          float64  // 单用户单日在线支付金额上限（元，PAYMENT_DAILY_LIMIT，0=不限），防止大额转账被渠道收取高额手续费
	AffCommissionRateFV        float64  // 人脸核验推广提成比例（AFF_COMMISSION_RATE_FV，0~1，默认 0.2），全局统一生效
	AffCommissionRateSMS       float64  // 短信推广提成比例（AFF_COMMISSION_RATE_SMS，0~1，默认 0.2），全局统一生效
	UploadURLTTLDays           int      // 上传图片对外地址签名有效期天数（UPLOAD_URL_TTL_DAYS，默认 365，需覆盖上游审核/复查周期）
	CallbackTrustIPs           []string // 上游回调信任 IP（联麓/FinAuth/支付宝/微信推送来源，支持精确 IP 与 CIDR，英文逗号分隔；未配置时放行）
	// 客服联系方式（存系统库 setting 分组 contact，后台「系统设置」维护；门户首页每次加载实时读取展示）
	Contact ContactConfig
	// 品牌信息（存系统库 setting 分组 brand，后台「系统设置」维护；各前端启动时经 /console/config 读取展示）
	Brand BrandConfig
}

// BrandConfig 品牌与域名（前端展示内容 + 站点主域，全部可在后台在线维护）
type BrandConfig struct {
	Name       string // 平台名称
	SubName    string // 平台副标题/简称
	ICP        string // ICP 备案号（页脚展示）
	Company    string // 公司主体
	Address    string // 公司地址（页脚展示）
	Copyright  string // 版权文案（页脚展示）
	LogoURL    string // Logo 图片地址（留空时前端展示文字品牌名）
	RootDomain string // 站点主域：各站点域名与对外地址由此拼装（www./console./api./img./service.）
}

// ContactConfig 客服联系方式（门户首页页脚展示，可留空表示不展示该项）
type ContactConfig struct {
	Email  string // 客服邮箱
	Phone  string // 客服电话
	Wechat string // 客服微信号
	QQ     string // 客服 QQ
	Hours  string // 服务时间
}

// StarLoftConfig StarLoft 平台开放 API 配置（OEM 系统的唯一上游）
type StarLoftConfig struct {
	APIKey    string // API Key（密钥类，只走 .env）
	APISecret string // API Secret（密钥类，只走 .env，用于请求签名与回调验签）
	BaseURL   string // API 基址（如 https://api.example.com，存系统库 setting）
	// MarketingEnabled 上游账号是否已开通营销短信通道：未开通时营销模板报备与营销短信发送不可用
	MarketingEnabled bool
}

// LogConfig 日志配置
type LogConfig struct {
	Dir string
}

type ServerConfig struct {
	Host string
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string // 系统库名（连接默认库）
	FvDBName     string // 人脸核验库名（跨库访问，人脸核验认证订单/资源包）
	MaxIdleConns int
	MaxOpenConns int
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

type JWTConfig struct {
	Secret      string
	ExpireHours int
	AdminSecret string
}

type TencentConfig struct {
	SecretID  string
	SecretKey string
	Captcha   TencentCaptchaConfig
}

type TencentCaptchaConfig struct {
	CaptchaAppID string
	AppSecretKey string
}

// AlipayConfig 支付宝开放平台支付配置（电脑网站支付 alipay.trade.page.pay）
type AlipayConfig struct {
	Enabled    int    // 启用开关：1-启用 2-不启用
	AppID      string // 应用AppID
	PrivateKey string // 应用私钥（RSA2，PEM）
	PublicKey  string // 支付宝公钥（PEM，用于回调验签）
}

// WechatPayConfig 微信支付（API v3）配置（Native 扫码 + H5 网页支付）
type WechatPayConfig struct {
	Enabled         int    // 启用开关：1-启用 2-不启用
	AppID           string // 公众号/开放平台 AppID
	MchID           string // 商户号
	ApiV3Key        string // APIv3 密钥（32 字节，用于回调解密）
	MerchantPrivKey string // 商户 API 私钥（PEM，用于请求签名）
	MchSerialNo     string // 商户 API 证书序列号
	PublicKey       string // 微信支付公钥（PEM，用于回调验签）
}

// WechatLoginConfig 微信一键登录配置。
// 与微信支付是两套独立的应用：手机端走公众号网页授权（MPAppID），PC 端走开放平台网站应用扫码（OpenAppID）；
// 两套 AppSecret 均为密钥类，只由 .env 提供；开关与 AppID 存系统库 setting。
// 注意：微信后台「网页授权域名」/「授权回调域」只允许登记少量域名（且为域名、不含路径），
// 回跳地址由 site.Platform().ConsoleBase() 推导，品牌主域变更后必须同步到微信后台重配，否则报 redirect_uri 参数错误。
type WechatLoginConfig struct {
	Enabled       int    // 启用开关：1-启用 2-不启用
	MPAppID       string // 公众号 AppID（手机端网页授权）
	MPAppSecret   string // 公众号 AppSecret（密钥，只走 .env）
	OpenAppID     string // 开放平台网站应用 AppID（PC 扫码登录）
	OpenAppSecret string // 开放平台网站应用 AppSecret（密钥，只走 .env）
}

// 账户实名人脸核身 provider 取值
const (
	FaceProviderStarLoft = "starloft" // 上游 StarLoft 平台开放 API（默认）
	FaceProviderTencent  = "tencent"  // 腾讯云人脸核身（备选）
)

// 企业工商四要素核验 provider 取值
const (
	EnterpriseVerifyProviderTencent = "tencent" // 腾讯云 OCR（默认）
	EnterpriseVerifyProviderAliyun  = "aliyun"  // 阿里云
)

// 人机验证码通道 provider 取值
const (
	CaptchaProviderTencent = "tencent" // 腾讯天御验证码（默认）
	CaptchaProviderGeetest  = "geetest" // 极验行为验证码
	CaptchaProviderAliyun   = "aliyun"  // 阿里云行为验证码
)

// PaymentChannelEnabled 在线支付渠道启用开关：1-启用 2-不启用。
// 未配置（0）按启用处理，渠道最终是否可用仍取决于凭据是否齐全（凭据缺失时客户端不构建、渠道不可用且不影响启动）。
func PaymentChannelEnabled(enabled int) bool {
	return enabled != 2
}

// Load 从环境变量加载所有配置
func Load() (*Config, error) {
	cfg := &Config{}

	// 从环境变量加载所有配置
	loadFromEnv(cfg)

	// 校验关键密钥（启动早失败，避免带空/弱密钥运行泄露账户或数据）
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate 校验自举必需密钥非空且满足最小长度。
// 说明：数据库/Redis 连接凭据与 JWT/数据加密密钥必须先于数据库可用，无法从数据库读取，故在此强校验；
// 第三方业务密钥（腾讯云/支付宝/微信/短信上游等）可由 oem_sys.setting 提供，
// 在数据库就绪后经 ApplySettingOverrides 覆盖，缺失时仅记日志提示能力不可用、不阻断启动。
func (cfg *Config) validate() error {
	checks := []struct {
		name string
		val  string
		min  int
	}{
		{"JWT_SECRET", cfg.JWT.Secret, 32},
		{"JWT_ADMIN_SECRET", cfg.JWT.AdminSecret, 32},
		{"DB_PASSWORD", cfg.Database.Password, 8},
		{"REDIS_PASSWORD", cfg.Redis.Password, 8},
		{"DATA_ENCRYPT_KEY", cfg.DataEncryptKey, 32},
	}
	for _, c := range checks {
		if len(c.val) < c.min {
			return fmt.Errorf("关键配置 %s 缺失或强度不足：至少需要 %d 个字符", c.name, c.min)
		}
	}
	return nil
}

// loadFromEnv 从环境变量加载所有配置
func loadFromEnv(cfg *Config) {
	// 服务器配置
	cfg.Server.Host = getEnv("SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = getEnv("SERVER_PORT", cfg.Server.Port)
	cfg.Server.Mode = getEnv("GIN_MODE", cfg.Server.Mode)

	// 数据库配置
	cfg.Database.Host = getEnv("DB_HOST", cfg.Database.Host)
	cfg.Database.Port = getEnvInt("DB_PORT", cfg.Database.Port)
	cfg.Database.User = getEnv("DB_USER", cfg.Database.User)
	cfg.Database.Password = getEnv("DB_PASSWORD", cfg.Database.Password)
	// 分库库名写死为固定常量（与迁移建库名一致，不支持通过环境变量修改）
	cfg.Database.DBName = model.SysDB
	cfg.Database.FvDBName = model.FvDB
	cfg.Database.MaxIdleConns = getEnvInt("DB_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns)
	cfg.Database.MaxOpenConns = getEnvInt("DB_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns)

	// Redis配置
	cfg.Redis.Host = getEnv("REDIS_HOST", cfg.Redis.Host)
	cfg.Redis.Port = getEnvInt("REDIS_PORT", cfg.Redis.Port)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", cfg.Redis.Password)
	cfg.Redis.DB = getEnvInt("REDIS_DB", cfg.Redis.DB)
	cfg.Redis.PoolSize = getEnvInt("REDIS_POOL_SIZE", cfg.Redis.PoolSize)

	// JWT配置
	cfg.JWT.Secret = getEnv("JWT_SECRET", cfg.JWT.Secret)
	cfg.JWT.AdminSecret = getEnv("JWT_ADMIN_SECRET", cfg.JWT.AdminSecret)
	cfg.JWT.ExpireHours = getEnvInt("JWT_EXPIRE_HOURS", cfg.JWT.ExpireHours)

	// StarLoft 平台开放 API（API Key/Secret 为密钥类，只走本文件；基址与营销开关见文末默认值块）
	cfg.StarLoft.APIKey = getEnv("STARLOFT_API_KEY", cfg.StarLoft.APIKey)
	cfg.StarLoft.APISecret = getEnv("STARLOFT_API_SECRET", cfg.StarLoft.APISecret)

	// 腾讯云配置（SecretId/SecretKey/验证码 AppSecretKey 为密钥类，只走本文件）
	cfg.Tencent.SecretID = getEnv("TENCENT_SECRET_ID", cfg.Tencent.SecretID)
	cfg.Tencent.SecretKey = getEnv("TENCENT_SECRET_KEY", cfg.Tencent.SecretKey)
	cfg.Tencent.Captcha.AppSecretKey = getEnv("TENCENT_CAPTCHA_SECRET", cfg.Tencent.Captcha.AppSecretKey)

	// 极验行为验证码密钥（密钥类）
	cfg.GeetestCaptchaKey = getEnv("GEETEST_CAPTCHA_KEY", cfg.GeetestCaptchaKey)

	// 阿里云账号密钥与云市场 AppCode（密钥类）
	cfg.AliyunAccessKeyID = getEnv("ALIYUN_ACCESS_KEY_ID", cfg.AliyunAccessKeyID)
	cfg.AliyunAccessKeySecret = getEnv("ALIYUN_ACCESS_KEY_SECRET", cfg.AliyunAccessKeySecret)
	cfg.AliyunMarketAppCode = getEnv("ALIYUN_MARKET_APPCODE", cfg.AliyunMarketAppCode)

	// 数据加密密钥（敏感字段存储加密）
	cfg.DataEncryptKey = getEnv("DATA_ENCRYPT_KEY", cfg.DataEncryptKey)

	// 证书目录（bind mount 宿主 ./certs，容器内 /app/certs）：PEM 证书只以文件形式存放并按路径读取，
	// 不在 .env 中填写证书内容。须在读取证书文件之前赋值。
	cfg.CertsDir = getEnv("CERTS_DIR", "/app/certs")

	// 支付宝支付配置（PEM 证书只按 certs 内的文件路径读取；启用开关与 AppID 见文末默认值块）
	cfg.Alipay.PrivateKey = loadPEMFile(cfg.CertsDir, getEnv("ALIPAY_PRIVATE_KEY_FILE", "alipay/app_private_key.pem"))
	cfg.Alipay.PublicKey = loadPEMFile(cfg.CertsDir, getEnv("ALIPAY_PUBLIC_KEY_FILE", "alipay/alipay_public_key.pem"))

	// 微信支付配置（API v3：仅 APIv3 密钥为密钥类走本文件；PEM 证书同上按路径读取）
	cfg.WechatPay.ApiV3Key = getEnv("WECHAT_API_V3_KEY", cfg.WechatPay.ApiV3Key)
	cfg.WechatPay.MerchantPrivKey = loadPEMFile(cfg.CertsDir, getEnv("WECHAT_MCH_PRIVATE_KEY_FILE", "wechat/mch_private_key.pem"))
	cfg.WechatPay.PublicKey = loadPEMFile(cfg.CertsDir, getEnv("WECHAT_PUBLIC_KEY_FILE", "wechat/wechat_public_key.pem"))

	// 微信一键登录（网页授权 / 扫码登录）：两套 AppSecret 为密钥类，只走本文件；开关与 AppID 见文末默认值块
	cfg.WechatLogin.MPAppSecret = getEnv("WECHAT_LOGIN_MP_APP_SECRET", cfg.WechatLogin.MPAppSecret)
	cfg.WechatLogin.OpenAppSecret = getEnv("WECHAT_LOGIN_OPEN_APP_SECRET", cfg.WechatLogin.OpenAppSecret)

	// 日志配置
	cfg.Log.Dir = getEnv("LOG_DIR", cfg.Log.Dir)

	// 上传目录（bind mount 宿主 ./data/uploads，容器内 /app/uploads）
	cfg.UploadDir = getEnv("UPLOAD_DIR", "/app/uploads")

	// 认证媒体目录（bind mount 宿主 ./data/media，容器内 /app/media）：人脸核验照片/视频
	cfg.MediaDir = getEnv("MEDIA_DIR", "/app/media")

	// 上传图片对外地址签名有效期（天），默认 365，需覆盖上游审核与复查周期
	cfg.UploadURLTTLDays = getEnvInt("UPLOAD_URL_TTL_DAYS", 365)
	if cfg.UploadURLTTLDays <= 0 {
		cfg.UploadURLTTLDays = 365
	}

	// 上游回调信任 IP（英文逗号分隔，支持精确 IP 与 CIDR）：StarLoft 平台/支付宝/微信推送来源；未配置时放行
	cfg.CallbackTrustIPs = splitList(getEnv("CALLBACK_TRUST_IPS", ""))

	// 非密钥业务配置（单价/成本、开关、AppID/商户号、地域、接口基址、提成比例等）只存数据库
	// （系统库 setting + 各产品库 product_config），不从 .env 读取；下列内置默认值仅用于数据库
	// 无值时的兜底与首次启动播种，须与后台默认一致。
	cfg.StarLoft.BaseURL = "https://api.starloft.cn"
	cfg.FaceProvider = FaceProviderStarLoft
	cfg.EnterpriseVerifyProvider = EnterpriseVerifyProviderTencent
	cfg.CaptchaProvider = CaptchaProviderTencent
	cfg.TencentRegion = "ap-guangzhou"
	cfg.Tencent.Captcha.CaptchaAppID = ""
	cfg.Alipay.Enabled = 1
	cfg.WechatPay.Enabled = 1
	cfg.WechatLogin.Enabled = 1
	cfg.SMSPrice = 0.05
	cfg.FvAuthPrice = 1.00
	cfg.FvSelfPrice = 0.80
	cfg.KycPersonalPrice = 1.00
	cfg.KycEnterprisePrice = 2.00
	cfg.AffCommissionRateFV = model.DefaultCommissionRate
	cfg.AffCommissionRateSMS = model.DefaultCommissionRate
	// 客服联系方式默认值（门户首页展示，后台可在线改；留空表示首页不展示该项）
	cfg.Contact.Email = "service@example.com"
	cfg.Contact.Phone = ""
	cfg.Contact.Wechat = ""
	cfg.Contact.QQ = ""
	cfg.Contact.Hours = "周一至周日 8:00 - 24:00"
	// 品牌与域名默认值（后台可在线改；站点主域用于拼装各站点域名与对外回调地址）
	cfg.Brand.Name = "OEM 云服务"
	cfg.Brand.SubName = ""
	cfg.Brand.ICP = ""
	cfg.Brand.Company = "OEM 网络科技有限公司"
	cfg.Brand.Address = ""
	cfg.Brand.Copyright = "© OEM 云服务"
	cfg.Brand.LogoURL = ""
	cfg.Brand.RootDomain = site.RootDomain
	// 各产品成本单价（FV_AUTH_COST / FV_SELF_COST / SMS_COST）默认 0（无成本），仅在后台产品配置中维护；
	// 支付日限额（PAYMENT_DAILY_LIMIT）默认 0（不限）、待支付订单过期分钟数（PAYMENT_EXPIRE_MINUTES）默认 30，
	// 仅在后台系统设置中维护，此处仅为表内无值时的兜底。
	cfg.PaymentExpireMinutes = 30

	// 推广提成比例（0~1，按产品分档：人脸核验 / 短信）默认 20%，见上方默认值块。
	// 两档各自独立配置，不再保留「单个变量同时管两档」的旧兼容分支：
	// 旧键 AFF_COMMISSION_RATE 已由迁移 000034 从系统设置中删除，代码不再读取。
}

// getEnv 读取环境变量，空值返回默认值
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getEnvInt 解析整数环境变量；缺失或非法时用默认值
func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// splitList 按英文逗号切分配置项，去掉首尾空白与空项（用于 IP 名单等逗号分隔配置）
func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadPEMFile 按路径读取 PEM 证书文件内容：path 为绝对路径时直接使用，否则相对 CertsDir 解析。
// 文件缺失或内容为空时返回空串（该渠道凭据不齐 → 客户端不构建、渠道不可用，仅记日志不阻断启动）。
func loadPEMFile(certsDir, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(certsDir, path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("证书文件读取失败（对应渠道将不可用）: %s: %v", path, err)
		return ""
	}
	s := strings.TrimSpace(string(content))
	if s == "" {
		log.Printf("证书文件内容为空（对应渠道将不可用）: %s", path)
		return ""
	}
	return s
}
