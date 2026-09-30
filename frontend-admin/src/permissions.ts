// 后台权限码与目录（与后端 config.PermissionCatalog 对齐；权限码常量见 backend/internal/model/permission.go）
export const PERMISSION_ALL = 'all'

// 写权限码后缀：读码 + 后缀 = 该模块的写权限（写隐含读）
export const WRITE_SUFFIX = '.write'

// 分组通配权限码列表（覆盖该分区全部读/写权限）
export const GROUP_WILDCARD_LIST = ['sys', 'sms', 'fv', 'ops']

// 分组名 → 分组通配码
export const GROUP_WILDCARDS: Record<string, string> = {
  平台管理: 'sys',
  短信服务: 'sms',
  人脸核验: 'fv',
  运维审计: 'ops'
}

// 通配码展示名
export const WILDCARD_LABELS: Record<string, string> = {
  [PERMISSION_ALL]: '全部权限',
  sys: '平台管理（全部）',
  sms: '短信服务（全部）',
  fv: '人脸核验（全部）',
  ops: '运维审计（全部）'
}

export interface PermissionSpec {
  code: string
  group: string
  label: string
  // 该模块是否有写操作（有则另有 `<code>.write` 可勾选）
  writable: boolean
}

// 全部可勾选权限模块（「员工管理」页按分组渲染）
export const PERMISSION_CATALOG: PermissionSpec[] = [
  { code: 'sys.dashboard', group: '平台管理', label: '数据统计', writable: false },
  { code: 'sys.users', group: '平台管理', label: '用户管理', writable: true },
  { code: 'sys.finance', group: '平台管理', label: '财务管理', writable: true },
  { code: 'sys.aff', group: '平台管理', label: '推广管理', writable: true },
  { code: 'sys.admins', group: '平台管理', label: '员工管理', writable: true },
  { code: 'sys.sales', group: '平台管理', label: '销售业绩', writable: false },
  { code: 'sys.settings', group: '平台管理', label: '系统设置', writable: true },

  { code: 'sms.stats', group: '短信服务', label: '数据统计', writable: false },
  { code: 'sms.signs', group: '短信服务', label: '签名审核', writable: true },
  { code: 'sms.templates', group: '短信服务', label: '模板审核', writable: true },
  { code: 'sms.records', group: '短信服务', label: '发送记录', writable: false },
  { code: 'sms.replies', group: '短信服务', label: '短信回复', writable: false },
  { code: 'sms.packs', group: '短信服务', label: '资源包管理', writable: true },
  { code: 'sms.product_config', group: '短信服务', label: '产品配置', writable: true },

  { code: 'fv.stats', group: '人脸核验', label: '数据统计', writable: false },
  { code: 'fv.records', group: '人脸核验', label: '认证记录', writable: true },
  { code: 'fv.packs', group: '人脸核验', label: '资源包管理', writable: true },
  { code: 'fv.product_config', group: '人脸核验', label: '产品配置', writable: true },

  { code: 'ops.logs', group: '运维审计', label: '日志与审计', writable: false },
  { code: 'ops.notify', group: '运维审计', label: '通知重试', writable: true },
  { code: 'ops.monitor', group: '运维审计', label: '系统监控', writable: false },
  { code: 'ops.user_packs', group: '运维审计', label: '用户资源包', writable: false },
  { code: 'ops.api_keys', group: '运维审计', label: 'API 密钥', writable: false },
  { code: 'ops.uploads', group: '运维审计', label: '上传文件', writable: false },
  { code: 'ops.promoters', group: '运维审计', label: '推广归属', writable: false }
]

// 权限码 → 展示名（读码 = 模块名；写码 = 模块名（可修改）；另含通配码）
export const PERMISSION_LABELS: Record<string, string> = (() => {
  const map: Record<string, string> = { ...WILDCARD_LABELS }
  for (const spec of PERMISSION_CATALOG) {
    map[spec.code] = spec.label
    if (spec.writable) {
      map[spec.code + WRITE_SUFFIX] = `${spec.label}（可修改）`
    }
  }
  return map
})()

// isWriteCode 是否写权限码
export const isWriteCode = (code: string) => code.endsWith(WRITE_SUFFIX)

// writeCode 读码 → 写码
export const writeCode = (code: string) => code + WRITE_SUFFIX

// readCodeOf 取模块读码（写码则去掉后缀）
export const readCodeOf = (code: string) => (isWriteCode(code) ? code.slice(0, -WRITE_SUFFIX.length) : code)

// groupOf 取权限码所属分组（第一个 '.' 前的前缀；通配码返回其自身）
export const groupOf = (code: string) => code.split('.')[0]

// isGroupWildcard 是否分组通配码
export const isGroupWildcard = (token: string) => GROUP_WILDCARD_LIST.includes(token)

// 登录后落地页优先级：取第一个有权限的页面（列表顺序即优先级）
export const LANDING_ORDER: { path: string; code: string }[] = [
  { path: '/sys/dashboard', code: 'sys.dashboard' },
  { path: '/sys/sales', code: 'sys.sales' },
  { path: '/sys/users', code: 'sys.users' },
  { path: '/sys/finance', code: 'sys.finance' },
  { path: '/sys/commissions', code: 'sys.aff' },
  { path: '/sys/admins', code: 'sys.admins' },
  { path: '/sys/settings', code: 'sys.settings' },
  { path: '/sys/monitor', code: 'ops.monitor' },
  { path: '/sys/notify', code: 'ops.notify' },
  { path: '/sys/logs/login', code: 'ops.logs' },
  { path: '/sys/api-keys', code: 'ops.api_keys' },
  { path: '/sys/uploads', code: 'ops.uploads' },
  { path: '/sys/promoters', code: 'ops.promoters' },
  { path: '/sms/stats', code: 'sms.stats' },
  { path: '/sms/signs', code: 'sms.signs' },
  { path: '/sms/templates', code: 'sms.templates' },
  { path: '/sms/records', code: 'sms.records' },
  { path: '/sms/replies', code: 'sms.replies' },
  { path: '/sms/packs', code: 'sms.packs' },
  { path: '/sms/product-config', code: 'sms.product_config' },
  { path: '/fv/stats', code: 'fv.stats' },
  { path: '/fv/records', code: 'fv.records' },
  { path: '/fv/packs', code: 'fv.packs' },
  { path: '/fv/product-config', code: 'fv.product_config' }
]