package model

import "strings"

// 平台产品与服务标识（人脸核验 FV / 短信 SMS）
//
// 产品层级模型：
//   - 一级产品 fv（人脸核验，Face Verification），其下为具体服务 fv_auth（有源）/ fv_self（无源）
//   - 一级产品 sms（短信服务，Short Message Service）
//
// 服务标识用于：
//   - API 路由组 ServiceAccessGuard 判权：以一级产品（大类）为权限单元判权
//   - 资源包按产品隔离：ResourcePack.product / UserResourcePack.product / AuthRecord.product 取值
const (
	// ProductFV 人脸核验（一级产品，API 权限大类单元）
	ProductFV = "fv"

	// ServiceFVAuth 有源人脸核验（人脸 + 公安库身份要素比对人像库）
	ServiceFVAuth = "fv_auth"
	// ServiceFVSelf 无源人脸核验（仅活体识别，不比对身份要素）
	ServiceFVSelf = "fv_self"

	// ServiceSMS 短信通道服务（下发核验结果 / 业务短信，API 权限大类单元）
	ServiceSMS = "sms"
	// ProductSMSMarketing 营销短信资源包标识（仅作资源包类型区分；计费单价/成本/提成档位仍按 ServiceSMS）
	ProductSMSMarketing = "sms_marketing"

	// 账户实名（Web）超次计费订单归属标识（资源包按该标识匹配，无对应资源包时走余额扣费）
	ServiceKYCPersonal   = "kyc_personal"   // 个人实名（免费次数用尽后按 KYC_PERSONAL_PRICE 计费）
	ServiceKYCEnterprise = "kyc_enterprise" // 企业实名（免费次数用尽后按 KYC_ENTERPRISE_PRICE 计费）
)

// 单用户定向定价（price_override）按产品分库存放：人脸核验与短信的覆盖价在各自产品库，
// 账户实名两档属平台账户能力、仍在系统库。
var priceOverrideTableNames = map[string]bool{
	SysDB + ".price_override": true,
	FvDB + ".price_override":  true,
	SmsDB + ".price_override": true,
}

// 各作用域下支持用户级定价的服务标识（与价格覆盖表所在库一致）
var (
	FvPricingServices  = []string{ServiceFVAuth, ServiceFVSelf}              // 人脸核验（存 FV 库）
	SmsPricingServices = []string{ServiceSMS}                                // 短信（存短信库）
	KycPricingServices = []string{ServiceKYCPersonal, ServiceKYCEnterprise}  // 账户实名（存系统库）
)

// PricingServicesByScope 按作用域取可定价服务标识：fv / sms 为产品页入口，kyc 为账户实名（入口在用户管理）
func PricingServicesByScope(scope string) []string {
	switch scope {
	case ProductFV:
		return FvPricingServices
	case ServiceSMS:
		return SmsPricingServices
	case SettingCategoryKYC:
		return KycPricingServices
	}
	return nil
}

// PriceOverrideTableByService 价格覆盖表（全限定名）：按服务标识取所在库的表
func PriceOverrideTableByService(service string) string {
	switch service {
	case ServiceFVAuth, ServiceFVSelf:
		return FvDB + ".price_override"
	case ServiceSMS, ProductSMSMarketing:
		return SmsDB + ".price_override"
	}
	return SysDB + ".price_override"
}

// IsValidPriceOverrideTable 校验价格覆盖表名（仓储拼接动态表名前必检，防注入）
func IsValidPriceOverrideTable(table string) bool { return priceOverrideTableNames[table] }

// API 端点权限标识（api.permission 的取值单元，逐个下游端点授权）。
// 权限为 "all"（全部端点）或逗号分隔的端点标识集合（如 fv_auth,sms_send）。
const (
	APIFvAuth    = "fv_auth"     // 有源人脸核验 POST /v1/fv/auth
	APIFvSelf    = "fv_self"     // 无源人脸核验 POST /v1/fv/self
	APIFvBestImg = "fv_best_img" // 活体最佳图领取 POST /v1/fv/best-img
	APIFvMedia   = "fv_media"    // 认证媒体下载 POST /v1/fv/media
	APIFvResult  = "fv_result"   // 认证结果查询 POST /v1/fv/result

	APISmsSend           = "sms_send"            // 短信发送 POST /v1/sms/send
	APISmsSign           = "sms_sign"            // 创建短信签名 POST /v1/sms/signs
	APISmsTemplateCreate = "sms_template_create" // 创建短信模板 POST /v1/sms/templates
	APISmsTemplateGet    = "sms_template_get"    // 查询短信模板 GET /v1/sms/templates/:id
	APISmsTemplateUpdate = "sms_template_update" // 修改短信模板 PUT /v1/sms/templates/:id
	APISmsTemplateDelete = "sms_template_delete" // 删除短信模板 DELETE /v1/sms/templates/:id
	APISmsReport         = "sms_report"          // 短信回执查询 POST /v1/sms/report
	APISmsReplies        = "sms_replies"         // 短信上行回复 POST /v1/sms/replies
)

// APIEndpoint 一个可授权的下游 API 端点
type APIEndpoint struct {
	Code         string // 权限标识（存入 api.permission）
	Title        string // 中文名称（控制台权限勾选展示）
	Product      string // 所属一级产品（fv / sms）
	NeedRealname int    // 调用所需实名等级（model.RealnamePersonal / RealnameEnterprise）
}

// APIEndpoints 可授权的下游 API 端点目录（控制台权限选择、创建校验、路由判权共用）
var APIEndpoints = []APIEndpoint{
	{APIFvAuth, "有源人脸核验", ProductFV, RealnameEnterprise},
	{APIFvSelf, "无源人脸核验", ProductFV, RealnameEnterprise},
	{APIFvBestImg, "活体最佳图领取", ProductFV, RealnameEnterprise},
	{APIFvMedia, "认证媒体下载", ProductFV, RealnameEnterprise},
	{APIFvResult, "认证结果查询", ProductFV, RealnameEnterprise},
	{APISmsSend, "短信发送", ServiceSMS, RealnamePersonal},
	{APISmsSign, "创建短信签名", ServiceSMS, RealnamePersonal},
	{APISmsTemplateCreate, "创建短信模板", ServiceSMS, RealnamePersonal},
	{APISmsTemplateGet, "查询短信模板", ServiceSMS, RealnamePersonal},
	{APISmsTemplateUpdate, "修改短信模板", ServiceSMS, RealnamePersonal},
	{APISmsTemplateDelete, "删除短信模板", ServiceSMS, RealnamePersonal},
	{APISmsReport, "短信发送回执查询", ServiceSMS, RealnamePersonal},
	{APISmsReplies, "短信上行回复查询", ServiceSMS, RealnamePersonal},
}

// APIEndpointByCode 按权限标识查端点定义
func APIEndpointByCode(code string) *APIEndpoint {
	for i := range APIEndpoints {
		if APIEndpoints[i].Code == code {
			return &APIEndpoints[i]
		}
	}
	return nil
}

// EndpointRealname 端点所需实名等级（未登记端点按最高等级处理）
func EndpointRealname(code string) int {
	if ep := APIEndpointByCode(code); ep != nil {
		return ep.NeedRealname
	}
	return RealnameEnterprise
}

// IsValidAPIPermission 判断权限范围是否合法：可为 "all"，或一个/多个以英文逗号分隔的端点标识（空白忽略）。
func IsValidAPIPermission(p string) bool {
	if p == "all" {
		return true
	}
	valid := false
	for _, part := range strings.Split(p, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if APIEndpointByCode(id) == nil {
			return false
		}
		valid = true
	}
	return valid
}

// APIAllows 判断密钥权限 permission 是否允许访问端点 code。
// "all" 放行全部；否则按逗号分隔的端点标识列表精确匹配。
func APIAllows(permission, code string) bool {
	if permission == "all" {
		return true
	}
	for _, part := range strings.Split(permission, ",") {
		if strings.TrimSpace(part) == code {
			return true
		}
	}
	return false
}
