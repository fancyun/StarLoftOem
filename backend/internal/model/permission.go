package model

// 后台账号权限码（admin_user.permissions，逗号分隔）。
// 命名约定：无后缀＝读权限（可查看该模块），`<模块>.write`＝写权限（可增删改，且隐含读）；
// 另有分组通配码 (sys/sms/fv/ops) 与 all。
const (
	PermissionAll = "all" // 全部权限（超级管理员，一账号是否超管只取决于是否持有 all，不依赖账号 id）

	// 分组通配权限码：持有该码即拥有该分组下全部读/写权限（含后续新增的页面）
	PermissionSysGroup = "sys" // 平台管理全部权限
	PermissionSmsGroup = "sms" // 短信服务全部权限
	PermissionFvGroup  = "fv"  // 人脸核验全部权限
	PermissionOpsGroup = "ops" // 运维审计全部权限

	// PermissionWriteSuffix 写权限码后缀：读码 + 后缀 = 该模块的写权限（如 sys.users.write）
	PermissionWriteSuffix = ".write"

	// 平台管理
	PermissionSysDashboard = "sys.dashboard" // 数据统计
	PermissionSysUsers     = "sys.users"     // 用户管理（含个人/企业实名、单价、状态、充值）
	PermissionSysFinance   = "sys.finance"   // 财务管理（财务统计/账单/支付记录）
	PermissionSysAff       = "sys.aff"       // 推广管理（推广商/提现审核）
	PermissionSysSettings  = "sys.settings"  // 系统设置
	PermissionSysAdmins    = "sys.admins"    // 员工管理
	PermissionSysSales     = "sys.sales"     // 销售业绩

	// 短信服务
	PermissionSmsStats         = "sms.stats"          // 数据统计
	PermissionSmsSigns         = "sms.signs"          // 签名审核（含公共签名标记）
	PermissionSmsTemplates     = "sms.templates"      // 模板审核
	PermissionSmsRecords       = "sms.records"        // 发送记录
	PermissionSmsReplies       = "sms.replies"        // 短信回复
	PermissionSmsPacks         = "sms.packs"          // 资源包管理
	PermissionSmsProductConfig = "sms.product_config" // 产品配置

	// 人脸核验
	PermissionFvStats         = "fv.stats"          // 数据统计
	PermissionFvRecords       = "fv.records"        // 认证记录
	PermissionFvPacks         = "fv.packs"          // 资源包管理
	PermissionFvProductConfig = "fv.product_config" // 产品配置

	// 运维审计（独立分组：不随 sys 通配授予，需显式勾选）
	PermissionOpsLogs      = "ops.logs"       // 日志与审计（登录日志、系统日志文件）
	PermissionOpsNotify    = "ops.notify"     // 通知重试（含手动重推）
	PermissionOpsMonitor   = "ops.monitor"    // 系统监控
	PermissionOpsUserPacks = "ops.user_packs" // 用户资源包（已购）
	PermissionOpsAPIKeys   = "ops.api_keys"   // API 密钥（只读）
	PermissionOpsUploads   = "ops.uploads"    // 上传文件
	PermissionOpsPromoters = "ops.promoters"  // 推广归属
)

// WritePermission 由读权限码派生写权限码（如 sys.users → sys.users.write）
func WritePermission(code string) string {
	return code + PermissionWriteSuffix
}
