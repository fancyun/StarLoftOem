import request from '@/utils/request'
import axios from 'axios'
import { useAdminStore } from '@/stores/admin'

// 加载受保护的上传图片（需 admin JWT，浏览器 <img> 直连不带鉴权会 403）：
// 对外地址（img.starloft.cn）与管理后台不同域，跨域携带 token 会被 CORS 拦截，
// 故统一改走同域 /img/uploads/*（由管理后台站点反代到后端 /img/uploads/*），
// 以 axios 携带 admin token 拉取 blob 并返回 object URL（由调用方在不再使用时 revokeObjectURL 释放）。
export async function loadAuthImage(url: string): Promise<string> {
  if (!url) return ''
  const idx = url.indexOf('/uploads/')
  if (idx < 0) return ''
  const relPath = url.slice(idx + '/uploads/'.length).replace(/^\/+/, '')
  if (!relPath) return ''
  const adminStore = useAdminStore()
  const resp = await axios.get(`/img/uploads/${relPath}`, {
    responseType: 'blob',
    timeout: 15000,
    headers: { Authorization: `Bearer ${adminStore.adminToken}` }
  })
  return URL.createObjectURL(resp.data)
}

// loadAuthFile 以管理员 token 拉取受保护文件（blob → objectURL），供认证媒体（照片/视频）查看。
// path 为后端绝对路径（如 /admin/records/1/media?kind=image），故不走 baseURL=/admin 的 request 实例。
// 媒体缺失时后端返回 HTTP 200 + JSON 错误体（blob type 为 application/json），此处直接抛错，
// 由调用方按「无该媒体」静默处理，避免把错误 JSON 当成媒体文件展示。
export async function loadAuthFile(path: string): Promise<string> {
  if (!path) return ''
  const adminStore = useAdminStore()
  const resp = await axios.get(path, {
    responseType: 'blob',
    timeout: 60000,
    headers: { Authorization: `Bearer ${adminStore.adminToken}` }
  })
  const type: string = resp.data?.type || ''
  if (type.includes('application/json')) {
    throw new Error('media not found')
  }
  return URL.createObjectURL(resp.data)
}

// 类型定义
interface StatsOverview {
  total_users: number
  today_orders: number
  today_revenue: number
  month_revenue: number
  today_payment_amount: number
  month_payment_amount: number
}

interface StatsOrdersResponse {
  dates: string[]
  counts: number[]
}

interface StatsRevenueResponse {
  dates: string[]
  amounts: number[]
}

// 管理后台API
export const adminAPI = {
  // 管理员登录
  login: (data: { username: string; password: string }) => {
    return request.post('/login', data)
  },

  // 获取用户列表
  getUsers: (params: any) => {
    return request.get('/users', { params })
  },

  // 更新用户状态
  updateUserStatus: (id: number, data: { status: number }) => {
    return request.put(`/users/${id}/status`, data)
  },

  // 重置用户免费实名次数（personal-个人 enterprise-企业）
  resetUserKycFree: (id: number, type: 'personal' | 'enterprise') => {
    return request.post(`/users/${id}/reset-kyc-free`, { type })
  },

  // 发放人脸核验测试资源包（product：fv_auth-有源 / fv_self-无源，默认 5 次，两者互不通用）
  grantFvTestPack: (data: { user_id: number; product: 'fv_auth' | 'fv_self'; count: number }) => {
    return request.post('/packs/grant-test', data)
  },

  // 发放短信测试资源包（product：sms-验证码/通知 / sms_marketing-营销，默认 20 条，两类互不通用）
  grantSmsTestPack: (data: { user_id: number; product: 'sms' | 'sms_marketing'; count: number }) => {
    return request.post('/sms/packs/grant-test', data)
  },

  // 手动注册用户
  registerUser: (data: { phone: string; username: string; password: string }) => {
    return request.post('/users/register', data)
  },

  // 为用户手动充值（需银行流水单号）
  rechargeUser: (id: number, data: { amount: number; bank_serial_no: string; remark?: string }) => {
    return request.post(`/users/${id}/recharge`, data)
  },

  // 获取用户财务统计
  getUserFinanceStats: (id: number) => {
    return request.get(`/users/${id}/finance/stats`)
  },

  // 获取用户余额流水
  getUserBalanceLogs: (id: number, params?: { page?: number; page_size?: number }) => {
    return request.get(`/users/${id}/balance-logs`, { params })
  },

  // 获取用户认证记录
  getUserAuthRecords: (id: number, params?: { page?: number; page_size?: number }) => {
    return request.get(`/users/${id}/auth-records`, { params })
  },

  // 获取认证记录列表
  getFvRecords: (params: any) => {
    return request.get('/records', { params })
  },

  // 获取支付订单列表
  getPayments: (params: any) => {
    return request.get('/payments', { params })
  },

  // 手动查询支付结果（向渠道查询真实状态并落地）
  queryPaymentResult: (id: number) => {
    return request.post(`/payments/${id}/query-result`)
  },

  // 个人实名（kyc）记录列表
  getKycRecords: (params: any) => {
    return request.get('/kyc', { params })
  },

  // 获取统计概览
  getStatsOverview: (): Promise<StatsOverview> => {
    return request.get('/stats/overview')
  },

  // 获取订单统计（按天数）
  getStatsOrders: (days?: number): Promise<StatsOrdersResponse> => {
    return request.get('/stats/orders', { params: { days: days || 7 } })
  },

  // 获取收入统计（按天数）；source=payment 按支付记录统计，默认按认证订单统计
  getStatsRevenue: (days?: number, source?: 'payment'): Promise<StatsRevenueResponse> => {
    return request.get('/stats/revenue', { params: { days: days || 7, source } })
  },

  // 获取最近认证记录
  getRecentRecords: (limit: number) => {
    return request.get(`/records/recent?limit=${limit}`)
  },

  // 获取用户详情
  getUserDetail: (id: number) => {
    return request.get(`/users/${id}`)
  },

  // 获取认证记录详情
  getRecordDetail: (id: number) => {
    return request.get(`/records/${id}`)
  },

  // 后台查询认证记录的上游结果（调上游核对并回写本地）
  queryFvRecordResult: (id: number) => {
    return request.post(`/records/${id}/query-result`)
  },

  // 获取企业实名（kyb）记录列表
  getKybRecords: (params: { page?: number; page_size?: number; status?: number }) => {
    return request.get('/kyb', { params })
  },

  // 后台企业实名开通（录入企业名称与统一社会信用代码）
  verifyKyb: (data: { user_id: number; company_name: string; credit_code: string }) => {
    return request.post('/kyb/verify', data)
  },

  // 后台人工审核企业实名申请（通过/驳回）
  reviewKybManual: (data: { id: number; action: 'approve' | 'reject'; reason?: string }) => {
    return request.post('/kyb/review', data)
  },

  // 获取财务汇总
  getFinanceSummary: (params: { start_date?: string; end_date?: string }) => {
    return request.get('/finance/summary', { params })
  },

  // 获取每日财务统计
  getFinanceDaily: (params: { start_date?: string; end_date?: string }) => {
    return request.get('/finance/daily', { params })
  },

  // 修改管理员密码
  changePassword: (data: { old_password: string; new_password: string }) => {
    return request.post('/change-password', data)
  },

  // 获取资源包列表
  getPacks: () => {
    return request.get('/packs')
  },

  // 创建资源包
  createPack: (data: { name: string; total_count: number; price: number; status?: number; product?: string; description?: string }) => {
    return request.post('/packs', data)
  },

  // 更新资源包
  updatePack: (id: number, data: any) => {
    return request.put(`/packs/${id}`, data)
  },

  // 删除（下架）资源包
  deletePack: (id: number) => {
    return request.delete(`/packs/${id}`)
  },

  // 短信资源包列表（短信库独立表；product 过滤类型：sms-验证码/通知 / sms_marketing-营销）
  getSmsPacks: (product?: 'sms' | 'sms_marketing') => {
    return request.get('/sms/packs', { params: product ? { product } : {} })
  },

  // 创建短信资源包
  createSmsPack: (data: { name: string; total_count: number; price: number; status?: number; product?: string; description?: string }) => {
    return request.post('/sms/packs', data)
  },

  // 更新短信资源包
  updateSmsPack: (id: number, data: any) => {
    return request.put(`/sms/packs/${id}`, data)
  },

  // 删除（下架）短信资源包
  deleteSmsPack: (id: number) => {
    return request.delete(`/sms/packs/${id}`)
  },

  // 短信签名后台审核列表
  listSmsSigns: (params: { status?: number; page?: number; page_size?: number }) => {
    return request.get('/sms/signs', { params })
  },

  // 后台审核签名
  reviewSmsSign: (id: number, data: { status: number; reason?: string }) => {
    return request.post(`/sms/signs/${id}/review`, data)
  },

  // 后台查询签名审核结果（调上游核对并回写本地）
  querySmsSign: (id: number) => {
    return request.post(`/sms/signs/${id}/query`)
  },

  // 修改签名审核结果/失败原因文本
  updateSmsSignResultMessage: (id: number, data: { result_message: string }) => {
    return request.put(`/sms/signs/${id}/result-message`, data)
  },

  // 设置签名的公共标记（0-私有 1-公共，仅改可见性，不动上游签名）
  setSmsSignPublic: (id: number, is_public: number) => {
    return request.put(`/sms/signs/${id}/public`, { is_public })
  },

  // 短信模板后台审核列表
  listSmsTemplates: (params: { status?: number; page?: number; page_size?: number }) => {
    return request.get('/sms/templates', { params })
  },

  // 后台审核模板
  reviewSmsTemplate: (id: number, data: { status: number; reason?: string }) => {
    return request.post(`/sms/templates/${id}/review`, data)
  },

  // 后台查询模板审核结果（调上游核对并回写本地）
  querySmsTemplate: (id: number) => {
    return request.post(`/sms/templates/${id}/query`)
  },

  // 全量短信发送记录（含用户手机号，日期筛选）
  getSmsRecords: (params: { page?: number; page_size?: number; start_date?: string; end_date?: string }) => {
    return request.get('/sms/records', { params })
  },

  // 全量短信统计（总条数/总金额 + 按日趋势）
  getSmsStats: (params: { days?: number }) => {
    return request.get('/sms/stats', { params })
  },

  // 全量短信上行回复（含用户手机号）
  getSmsReplies: (params: { page?: number; page_size?: number }) => {
    return request.get('/sms/replies', { params })
  },

  // 统一账单列表
  getBills: (params: { user_id?: number; product?: string; bill_type?: number; spend_type?: string; page?: number; page_size?: number }) => {
    return request.get('/bills', { params })
  },

  // 提成记录（用户型推广与员工销售两表合并；referrer_type/biz_type/keyword 与日期段筛选）
  listCommissions: (params: { referrer_type?: string; biz_type?: string; keyword?: string; start_date?: string; end_date?: string; page?: number; page_size?: number }) => {
    return request.get('/promotions/commissions', { params })
  },

  // 推广提现申请列表（referrer_type/referrer_id/status 不传表示全部）
  listPromotionWithdraws: (params: { referrer_type?: string; referrer_id?: number; status?: number; page?: number; page_size?: number }) => {
    return request.get('/promotion-withdraws', { params })
  },

  // 审核推广提现（approve 为通过，否则驳回并记录原因；仅线下渠道需要）
  reviewPromotionWithdraw: (id: number, data: { approve: boolean; reject_reason?: string }) => {
    return request.put(`/promotion-withdraws/${id}/review`, data)
  },

  // 标记推广提现已线下打款（仅线下渠道需要）
  markPromotionWithdrawPaid: (id: number) => {
    return request.put(`/promotion-withdraws/${id}/paid`)
  },

  // 账户实名两档的用户定向定价（人脸核验/短信的定向定价见产品配置页接口）
  getUserPrices: (id: number) => {
    return request.get(`/users/${id}/prices`)
  },

  // 保存账户实名两档的用户定向定价（items 写入，deleted 删除后回落平台价）
  setUserPrices: (id: number, data: { items: { price_type: 'unit'; target: string; price: number }[]; deleted: { price_type: 'unit'; target: string }[] }) => {
    return request.put(`/users/${id}/prices`, data)
  },

  // 产品维度的用户定向定价：按手机号/用户名检索用户（选人下拉）
  getProductPriceUsers: (params: { product: string; keyword?: string }) => {
    return request.get('/product-config/users', { params })
  },

  // 产品维度的用户定向定价：平台价 / 当前生效价 / 自定义价
  getProductPrices: (params: { product: string; user_id: number }) => {
    return request.get('/product-config/user-prices', { params })
  },

  // 保存产品维度的用户定向定价（items 写入，deleted 删除后回落平台价）
  setProductPrices: (data: {
    product: string
    user_id: number
    items: { price_type: 'unit'; target: string; price: number }[]
    deleted: { price_type: 'unit'; target: string }[]
  }) => {
    return request.put('/product-config/user-prices', data)
  },

  // 系统配置列表（category 不传表示全部）
  getSettings: (params?: { category?: string }) => {
    return request.get('/settings', { params })
  },

  // 新增/更新系统配置（敏感键提交空值表示不修改）
  upsertSetting: (data: { config_key: string; config_value: string; category: string; remark: string }) => {
    return request.put('/settings', data)
  },

  // 删除系统配置
  deleteSetting: (configKey: string) => {
    return request.delete('/settings', { params: { config_key: configKey } })
  },

  // 产品配置列表（product 缺省返回全部产品）
  getProductConfigs: (product?: 'fv' | 'sms') => {
    return request.get('/product-config', { params: { product } })
  },

  // 新增/更新产品配置
  upsertProductConfig: (data: { product: 'fv' | 'sms'; config_key: string; config_value: string; remark: string }) => {
    return request.put('/product-config', data)
  },

  // 删除产品配置
  deleteProductConfig: (product: 'fv' | 'sms', configKey: string) => {
    return request.delete('/product-config', { params: { product, config_key: configKey } })
  },

  // 当前登录账号信息（含权限清单）
  me: () => {
    return request.get('/me')
  },

  // 员工管理：后台账号列表
  listAdmins: (params?: { page?: number; page_size?: number }) => {
    return request.get('/admins', { params })
  },

  // 员工管理：新增后台账号
  createAdmin: (data: { username: string; password: string; nickname?: string; permissions: string[] }) => {
    return request.post('/admins', data)
  },

  // 员工管理：修改账号昵称/状态/权限
  updateAdmin: (id: number, data: { nickname?: string; status?: number; permissions?: string[] }) => {
    return request.put(`/admins/${id}`, data)
  },

  // 员工管理：重置账号密码
  resetAdminPassword: (id: number, password: string) => {
    return request.put(`/admins/${id}/password`, { password })
  },

  // 销售月度报告：概览（staff_id 缺省为当前登录账号，month 缺省为当前月）
  getSalesSummary: (staffId?: number, month?: string) => {
    return request.get('/sales/summary', { params: { staff_id: staffId, month } })
  },

  // 销售月度报告：提成流水（按月份过滤）
  getSalesCommissions: (params: { staff_id?: number; month?: string; page?: number; page_size?: number }) => {
    return request.get('/sales/commissions', { params })
  },

  // 销售月度报告：推广客户（按注册月份过滤）
  getSalesUsers: (params: { staff_id?: number; month?: string; page?: number; page_size?: number }) => {
    return request.get('/sales/users', { params })
  },

  // 运维审计：用户登录日志（keyword 匹配账号/IP）
  getUserLoginLogs: (params: any) => {
    return request.get('/login-logs/users', { params })
  },

  // 运维审计：管理员登录日志（keyword 匹配账号/IP）
  getAdminLoginLogs: (params: any) => {
    return request.get('/login-logs/admins', { params })
  },

  // 运维审计：可查看的日志文件列表（名称/大小/最后写入时间）
  getLogFiles: () => {
    return request.get('/logs/files')
  },

  // 运维审计：读取日志文件尾部若干行（可选关键词过滤）
  getLogFile: (name: string, params: { lines?: number; keyword?: string }) => {
    return request.get(`/logs/files/${name}`, { params })
  },

  // 运维审计：系统监控汇总（数据库/Redis/进程/日志/库表/运行配置）
  getSystemMonitor: () => {
    return request.get('/system/monitor')
  },

  // 运维审计：业务规模指标（后台账号数/配置项数/通知重试积压）
  getBusinessStats: (): Promise<any> => request.get('/stats/business'),

  // 运维审计：下游通知重试记录
  getNotifyRecords: (params: any) => {
    return request.get('/notify-records', { params })
  },

  // 运维审计：手动重推一条通知（以业务类型 + 业务单号定位；force=true 时重置已放弃记录后再推）
  retryNotifyRecord: (data: { biz_type: string; biz_no: string; force: boolean }) => {
    return request.post('/notify-records/retry', data)
  },

  // 用户资产：用户已购资源包（scope=fv|sms 留空合并）
  getUserPacks: (params: any) => {
    return request.get('/user-packs', { params })
  },

  // 用户资产：全平台 API 密钥（只读，不含 api_secret）
  getAdminApiKeys: (params: any) => {
    return request.get('/api-keys', { params })
  },

  // 用户资产：上传文件登记
  getUploads: (params: any) => {
    return request.get('/uploads', { params })
  },

  // 推广归属：推广商列表（用户型推广 + 员工销售）
  getPromoters: (params: any) => {
    return request.get('/promoters', { params })
  },

  // 推广归属：推广商详情（基本信息 + 上级 + 下级用户分页）
  getPromoterDetail: (type: string, id: number, params: any) => {
    return request.get(`/promoters/${type}/${id}`, { params })
  }
}
