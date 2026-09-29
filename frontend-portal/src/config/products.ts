// 平台产品目录配置（一级产品模型）
// 一级产品：key（URL 路径段，即产品简称，对应 /product/xxx）、name、english（英文名）、desc、icon、status，
// 以及渲染详情页用的 tagline/features/scenarios/consolePath。
// 每个产品均复用 ProductPage.vue 渲染；菜单由 SiteLayout 依据本目录自动生成。

export interface ProductFeature {
  title: string
  desc: string
}

/** 一级产品，如 fv（人脸核验）、sms（短信服务） */
export interface Product {
  /** URL 一级路径段，如 fv、sms（对应 /product/xxx） */
  key: string
  name: string
  english: string
  desc: string
  /** available 正常使用 / coming-soon 建设中 */
  status: 'available' | 'coming-soon'
  /** 卡片图标（内联 SVG 字符串） */
  icon: string
  tagline?: string
  features?: ProductFeature[]
  scenarios?: string[]
  /** 控制台内对应产品页地址 */
  consolePath?: string
}

/* ---------- 内联 SVG 图标 ---------- */

const faceIcon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11a1 1 0 1 0 0-2 1 1 0 0 0 0 2z"/><path d="M15 11a1 1 0 1 0 0-2 1 1 0 0 0 0 2z"/><path d="M8 15a4 4 0 0 0 8 0"/><circle cx="12" cy="12" r="9"/></svg>'
const smsIcon =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/></svg>'

/* ---------- 产品目录 ---------- */

export const products: Product[] = [
  {
    key: 'fv',
    name: '人脸核验',
    english: 'Face Verification',
    desc: '基于面部特征的活体核身与人脸比对服务，覆盖有源/无源两种核身模式。',
    status: 'available',
    icon: faceIcon,
    tagline: '活体人脸 · 有源/无源比对 · 扫码承接',
    features: [
      { title: '有源核验 (fv_auth)', desc: '人脸 + 公安库真实身份比对，适用于实名开户、风控核验' },
      { title: '无源核验 (fv_self)', desc: '人脸与本人留底照片比对，适用于本人操作确认' },
      { title: '活体检测', desc: '集成活体识别能力，防范照片、视频等攻击手段' },
      { title: '扫码承接', desc: 'PC 展示二维码、手机扫码完成核验，流程顺畅' }
    ],
    scenarios: ['实名开户', '账户核验', '风控反欺诈', '本人操作确认', '业务实名', '直播平台'],
    consolePath: '/fv'
  },
  {
    key: 'sms',
    name: '短信服务',
    english: 'Short Message Service',
    desc: '验证码/通知/签名短信，高到达率、实时回执。',
    status: 'available',
    icon: smsIcon,
    tagline: '验证码短信 · 通知短信 · 签名提交',
    features: [
      { title: '验证码短信', desc: '登录、注册、找回密码等验证码场景快速触达' },
      { title: '通知短信', desc: '订单、告警等业务通知实时下发' },
      { title: '短信签名提交', desc: '在线提交签名模板，审核通过后即可发送' },
      { title: '状态回执', desc: '实时获取发送状态，失败可追踪' },
      { title: '内容审核', desc: '发送内容安全审核，规避违规风险' },
      { title: '资源包计费', desc: '短信按资源包预购，先扣资源包、余额兜底' }
    ],
    scenarios: ['注册验证', '登录验证', '通知提醒', '订单通知'],
    consolePath: '/sms'
  }
]

/** 按一级 key（含小写规范化）查找一级产品 */
export const productByKey = (key: string): Product | undefined => {
  const k = key.toLowerCase()
  return products.find((p) => p.key.toLowerCase() === k)
}

/** 解析产品详情页入口：仅支持一级（'fv'、'sms'）。找不到返回 undefined。 */
export const resolveEntry = (entryKey: string): Product | undefined => {
  return productByKey(String(entryKey).split('/').filter(Boolean)[0] || '')
}