/**
 * 格式化后端返回的日期时间字符串。
 * 去除末尾时区信息（如 +08:00 / Z / +0800），并将 T 分隔符统一为空格。
 * 返回 "YYYY-MM-DD HH:mm:ss" 格式，若无值则返回 '-'。
 */
export function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  let s = String(value).trim()
  if (s === '') return '-'

  // 统一 T 分隔符为空格
  s = s.replace('T', ' ')

  // 去除毫秒部分
  s = s.replace(/\.\d+/, '')

  // 去除末尾时区：+08:00 / +0800 / -05:00 / Z
  s = s.replace(/\s*[+-]\d{2}:?\d{2}$/, '')
  s = s.replace(/\s*Z$/i, '')

  return s
}

// 可定价的服务标识，按作用域分组（各产品页只维护本产品的服务；账户实名两档入口在用户管理）
export const fvServiceTargets = ['fv_auth', 'fv_self']
export const smsServiceTargets = ['sms']
export const kycServiceTargets = ['kyc_personal', 'kyc_enterprise']

// 服务标识展示名
const serviceLabels: Record<string, string> = {
  fv_auth: '有源人脸核验',
  fv_self: '无源人脸核验',
  kyc_personal: '个人实名',
  kyc_enterprise: '企业实名',
  sms: '短信'
}

export function serviceLabel(target: string): string {
  return serviceLabels[target] || target
}

// 推广结算业务类型展示名
const settlementBizLabels: Record<string, string> = {
  pack_purchase: '购买资源包',
  sms_send: '短信发送',
  fv_auth: '人脸核验',
  kyc: '实名认证',
  refund: '退款冲回'
}

export function settlementBizLabel(biz: string): string {
  return settlementBizLabels[biz] || biz
}