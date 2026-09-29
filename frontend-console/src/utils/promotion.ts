import { computed, reactive } from 'vue'

// 平台展示名与站点域名（白标已移除：站点一律为平台自有域名，不再按访问域名解析）
export const PLATFORM_NAME = '星楼网络'
export const PLATFORM_SUB_NAME = 'StarLoft'

/** 站点域名组（恒为平台默认站点） */
export interface PromotionSites {
  portal: string
  console: string
  api: string
  img: string
  service: string
}

export interface ResolvedPromotion {
  promotion_id: number
  name: string
  domain: string
  sites: PromotionSites
  unit_prices: Record<string, number>
}

const PLATFORM_SITES: PromotionSites = {
  portal: 'https://www.starloft.cn',
  console: 'https://console.starloft.cn',
  api: 'https://api.starloft.cn',
  img: 'https://img.starloft.cn',
  service: 'https://service.starloft.cn'
}

// 站点品牌与域名状态：白标已移除，恒为平台直营（保留响应式状态以兼容既有引用）
export const siteState = reactive<{ matched: boolean; name: string; sites: PromotionSites }>({
  matched: false,
  name: '',
  sites: { ...PLATFORM_SITES }
})

/** 站点展示品牌名：恒为平台品牌 */
export const siteName = computed<string>(() => PLATFORM_NAME)

/** 站点英文副品牌：恒为平台副品牌 */
export const siteSubName = computed<string>(() => PLATFORM_SUB_NAME)

/** 取站点基地址（不含末尾斜杠），如 siteBase('console') → https://console.starloft.cn */
export const siteBase = (kind: keyof PromotionSites): string => siteState.sites[kind].replace(/\/$/, '')

/** 来源域名：白标已移除，恒为空串 */
export function promotionSourceDomain(): string {
  return ''
}

// 标签页标题：平台态固定为「星楼网络控制台」
document.title = `${PLATFORM_NAME}控制台`

/** 站点归属解析：白标已移除，恒按平台直营返回空结果 */
export function resolvePromotion(): Promise<ResolvedPromotion | null> {
  return Promise.resolve(null)
}