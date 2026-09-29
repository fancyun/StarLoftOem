import { computed, reactive } from 'vue'

import { brandState, siteBase as brandSiteBase } from './brand'

/** 站点域名组 */
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

// 品牌与站点域名：由后端公开配置下发（后台「系统设置 → 品牌与域名」维护），未取到时回落默认值。
// 保留响应式状态以兼容既有引用（siteState）。
export const siteState = reactive<{ matched: boolean; name: string; sites: PromotionSites }>({
  matched: false,
  name: '',
  sites: { ...brandState.sites }
})

/** 品牌配置加载后同步到 siteState（由应用入口在 loadBrand 完成后调用） */
export function syncBrandToSiteState() {
  siteState.name = brandState.name
  siteState.sites = { ...brandState.sites }
}

/** 平台展示名 */
export const PLATFORM_NAME = computed<string>(() => brandState.name)

/** 平台副品牌/副标题 */
export const PLATFORM_SUB_NAME = computed<string>(() => brandState.sub_name)

/** 站点展示品牌名 */
export const siteName = computed<string>(() => brandState.name)

/** 站点副品牌/副标题 */
export const siteSubName = computed<string>(() => brandState.sub_name)

/** 取站点基地址（不含末尾斜杠），如 siteBase('console') → https://console.starloft.cn */
export const siteBase = (kind: keyof PromotionSites): string => brandSiteBase(kind)

/** 来源域名：白标已移除，恒为空串 */
export function promotionSourceDomain(): string {
  return ''
}

// 标签页标题：品牌名 + 控制台（品牌配置加载后由 main.ts 再次刷新）
document.title = `${PLATFORM_NAME.value}控制台`

/** 站点归属解析：白标已移除，恒按平台直营返回空结果 */
export function resolvePromotion(): Promise<ResolvedPromotion | null> {
  return Promise.resolve(null)
}