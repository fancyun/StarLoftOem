import request from '@/utils/request'

// 类型定义
interface StatsOrdersResponse {
  dates: string[]
  counts: number[]
}

// 公开API（无需登录）
export const publicAPI = {
  // 获取公开配置（验证码 AppID / Kyc 单价）
  getConfig: () => {
    return request.get('/config')
  }
}

// 用户相关API
export const userAPI = {
  // 发送短信验证码
  sendSMSCode: (data: { phone: string; captcha_payload: Record<string, string>; scene: string }) => {
    return request.post('/send-code', data)
  },

  // 用户注册（手机号+用户名+短信验证码）
  // 归属：ref 为推广链接的 12 位推广码（优先），domain 为当前站点域名（回退），均由后端解析绑定
  register: (data: { phone: string; username: string; sms_code: string; password: string; captcha_payload: Record<string, string>; domain?: string; ref?: string }) => {
    return request.post('/register', data)
  },

  // 用户登录（支持用户名/手机号）
  login: (data: { account?: string; password?: string; sms_code?: string; login_type: string; captcha_payload: Record<string, string> }) => {
    return request.post('/login', data)
  },

  // 微信一键登录：发起公众号网页授权（仅在微信内置浏览器内有效，返回 authorize_url 供跳转）
  wechatAuthorize: (scene?: string): Promise<any> => {
    return request.get('/wechat/authorize', { params: scene ? { scene } : {} })
  },

  // 微信一键登录：用一次性票据换取登录态（票据由回调中转页或扫码轮询携带）
  wechatTicket: (data: { ticket: string }) => {
    return request.post('/wechat/ticket', data)
  },

  // 微信一键登录：未绑定时用手机号 + 短信验证码绑定已有账号
  wechatBind: (data: { bind_ticket: string; phone: string; sms_code: string }) => {
    return request.post('/wechat/bind', data)
  },

  // PC 扫码登录：创建扫码会话（返回 qr_ticket / qr_url / expires_in / poll_interval）
  createWechatQRSession: (): Promise<any> => {
    return request.post('/wechat/qr-session')
  },

  // PC 扫码登录：轮询会话状态（status: pending / need_bind / authorized / expired）
  wechatQRPoll: (data: { qr_ticket: string }): Promise<any> => {
    return request.post('/wechat/qr-poll', data)
  },

  // 微信绑定状态（登录态，返回公众号是否已绑定与昵称）
  getWechatBinding: (): Promise<any> => {
    return request.get('/wechat/binding')
  },

  // 发起微信绑定（登录态，公众号网页授权仅在微信内置浏览器内有效，返回 authorize_url）
  wechatBindAuthorize: (scene: string): Promise<any> => {
    return request.get('/wechat/bind-authorize', { params: { scene } })
  },

  // PC 扫码绑定：创建扫码绑定会话（登录态，返回 qr_ticket / qr_url / expires_in / poll_interval）
  createWechatQRBindSession: (): Promise<any> => {
    return request.post('/wechat/qr-bind-session')
  },

  // 解除微信绑定（scene: mp-公众号）
  unbindWechat: (scene: string) => {
    return request.delete('/wechat/binding', { params: { scene } })
  },

  // 获取用户信息
  getProfile: () => {
    return request.get('/profile')
  },

  // 查询Kyc认证状态
  getKycStatus: () => {
    return request.get('/kyc/status')
  },

  // Web端发起Kyc认证
  startKyc: (data: { name: string; id_card: string; return_url: string }) => {
    return request.post('/kyc', data)
  },

  // 取消当前进行中的认证
  cancelKyc: () => {
    return request.delete('/kyc')
  },

  // 同步上游认证结果
  syncKyc: () => {
    return request.post('/kyc/sync')
  },

  // 查询企业实名（kyb）状态
  getKybStatus: () => {
    return request.get('/kyb/status')
  },

  // Web端发起企业实名认证（法人扫脸）
  startKyb: (data: {
    company_name: string
    credit_code: string
    legal_name: string
    legal_id_card: string
    return_url: string
  }) => {
    return request.post('/kyb', data)
  },

  // 提交企业实名人工审核申请（未配置工商四要素核验能力时使用）
  submitKybManual: (data: {
    company_name: string
    credit_code: string
    legal_name: string
    legal_id_card: string
  }) => {
    return request.post('/kyb/manual', data)
  },

  // 查询人脸核验认证记录（FV 认证订单）
  getFvRecords: (params: { page?: number; page_size?: number; status?: string }) => {
    return request.get('/records', { params })
  },

  // 在线发起人脸核验（控制台 Web：返回自站核验链接，用户在手机端完成核身）
  startFv: (data: { product: string; name?: string; id_card?: string; return_url?: string }) => {
    return request.post('/fv', data)
  },

  // 查询认证调用统计（近30天，按天计数）
  getCallStats: (): Promise<StatsOrdersResponse> => {
    return request.get('/stats/calls')
  },

  // 发起充值
  createRecharge: (data: { amount: number; channel: string; scene?: string }) => {
    return request.post('/recharge', data)
  },

  // 查询充值结果（轮询）
  getRechargeResult: (params: { pay_order_no: string }) => {
    return request.get('/recharge/result', { params })
  },

  // 查询可提现（可退款）的充值支付订单与总额
  getRefundableOrders: () => {
    return request.get('/withdraw/refundable')
  },

  // 发起提现（按充值支付订单原路退款，可部分提现/自动拆分多个订单）
  withdraw: (data: { amount: number }) => {
    return request.post('/withdraw', data)
  },

  // API 密钥列表（含可授权端点目录与数量上限）
  listAPIKeys: (): Promise<any> => {
    return request.get('/api-keys')
  },

  // 创建 API 密钥（scope_type: all-全部接口 partial-部分接口，partial 时传 endpoints）
  createAPIKey: (data: { name: string; scope_type: string; endpoints?: string[] }): Promise<any> => {
    return request.post('/api-keys', data)
  },

  // 修改 API 密钥（名称与权限范围）
  updateAPIKey: (id: number, data: { name: string; scope_type: string; endpoints?: string[] }): Promise<any> => {
    return request.put(`/api-keys/${id}`, data)
  },

  // 删除 API 密钥
  deleteAPIKey: (id: number): Promise<any> => {
    return request.delete(`/api-keys/${id}`)
  },

  // 修改密码
  changePassword: (data: { sms_code: string; new_password: string; captcha_payload: Record<string, string> }) => {
    return request.post('/change-password', data)
  },

  // 获取在售资源包列表（可选按 product 筛选）
  listPacks: (product?: string) => {
    return request.get('/packs', { params: product ? { product } : {} })
  },

  // 使用余额购买资源包
  purchasePack: (id: number) => {
    return request.post(`/packs/${id}/purchase`)
  },

  // 在线购买资源包（余额 + 支付宝/微信组合支付）
  payPack: (id: number, data: { channel: string; scene?: string }) => {
    return request.post(`/packs/${id}/pay`, data)
  },

  // 我的资源包列表
  myPacks: () => {
    return request.get('/packs/mine')
  }
}

// 短信服务API（产品独立控制台）
export const smsAPI = {
  // 提交短信签名（含资质材料与联麓报备字段）
  submitSign: (data: {
    sign_name: string
    label?: number
    company?: string
    legal_person?: string
    credit_code?: string
    credit_user_name?: string
    id_card?: string
    phone?: string
    credit_code_url: string
    id_card_front: string
    id_card_back: string
    sx_commits?: string
    screenshot?: string
  }) => {
    return request.post('/sms/sign', data)
  },

  // 上传图片（营业执照/身份证等），返回可公网访问的图片 URL
  upload: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request.post('/upload', fd)
  },

  // 签名申请列表
  listSigns: (params: { page?: number; page_size?: number }) => {
    return request.get('/sms/signs', { params })
  },

  // 签名详情（修改页回填表单）
  getSign: (id: number) => {
    return request.get(`/sms/signs/${id}`)
  },

  // 手动查询签名审核状态（调上游核对并回写本地）
  querySignStatus: (id: number) => {
    return request.post(`/sms/signs/${id}/query`)
  },

  // 修改签名（创建新签名+删除旧签名，复用本地记录）
  updateSign: (id: number, data: {
    sign_name: string
    label?: number
    company: string
    legal_person: string
    credit_code: string
    credit_user_name: string
    id_card: string
    phone: string
    credit_code_url: string
    id_card_front: string
    id_card_back: string
    sx_commits?: string
    screenshot?: string
    auth_letter: string
  }) => {
    return request.put(`/sms/signs/${id}`, data)
  },

  // 提交模板（绑定已审核签名后报备上游，本地存表待审核）
  createTemplate: (data: {
    template_name: string
    template_content: string
    template_type: string
    sign_id?: number
  }) => {
    return request.post('/sms/templates', data)
  },

  // 修改模板（重新报备上游并重置待审核）
  updateTemplate: (
    id: number,
    data: {
      template_name: string
      template_content: string
      template_type: string
      sign_id?: number
    }
  ) => {
    return request.put(`/sms/templates/${id}`, data)
  },

  // 模板列表
  listTemplates: (params: { page?: number; page_size?: number }) => {
    return request.get('/sms/templates', { params })
  },

  // 模板详情（修改页回填表单）
  getTemplate: (id: number) => {
    return request.get(`/sms/templates/${id}`)
  },

  // 手动查询模板审核状态（调上游核对并回写本地）
  queryTemplateStatus: (id: number) => {
    return request.post(`/sms/templates/${id}/query`)
  },

  // 回填模板 ID（上游控制台创建后填写）
  fillTemplateID: (id: number, data: { template_id: string }) => {
    return request.post(`/sms/templates/${id}/template-id`, data)
  },

  // 发送记录（分页 + 日期筛选）
  listSendRecords: (params: { page?: number; page_size?: number; start_date?: string; end_date?: string }) => {
    return request.get('/sms/records', { params })
  },

  // 在线发送短信（控制台直接发送，按自己账号的资源包/余额计费）
  sendSMS: (data: { phone_number_set: string[]; template_id: string; template_params?: string[]; sign_name?: string }) => {
    return request.post('/sms/send', data)
  },

  // 短信统计（总条数/总金额 + 按日趋势）
  getSMSStats: (params: { days?: number }) => {
    return request.get('/sms/stats', { params })
  },

  // 在售短信资源包列表（短信库独立表）
  listPacks: () => {
    return request.get('/sms/packs')
  },

  // 使用余额购买短信资源包
  purchasePack: (id: number) => {
    return request.post(`/sms/packs/${id}/purchase`)
  },

  // 我的短信资源包列表
  myPacks: () => {
    return request.get('/sms/packs/mine')
  }
}

// 推广 API（打开推广页即自动开通推广码，按下级成交额获取提成收益，并可申请提现）
export const promotionAPI = {
  // 我的推广概览（推广码/推广链接/推广客户数/余额/生效单价/提成收益）
  profile: () => {
    return request.get('/promotions/me')
  },

  // 我的推广用户列表
  subUsers: (params: { page?: number; page_size?: number }) => {
    return request.get('/promotions/users', { params })
  },

  // 收益概览（累计提成 / 已提现 / 可提现 / 待审核提现笔数）
  finance: () => {
    return request.get('/promotions/finance')
  },

  // 提成流水分页
  financeLogs: (params: { page?: number; page_size?: number }) => {
    return request.get('/promotions/finance-logs', { params })
  },

  // 我的提现申请分页
  withdraws: (params: { page?: number; page_size?: number }) => {
    return request.get('/promotions/withdraws', { params })
  },

  // 发起提现（channel 为提现方式，当前仅 balance-提现到余额：站内划转、即时到账、免手续费）
  withdraw: (data: { amount: number; channel: string; payee_info?: string }) => {
    return request.post('/promotions/withdraw', data)
  }
}
