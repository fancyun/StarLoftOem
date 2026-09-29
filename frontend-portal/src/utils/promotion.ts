// 站点品牌与站点域名：白标已移除，门户一律展示平台品牌与平台站点域名。

import { computed } from 'vue'

/** 站点域名组（恒为平台默认站点） */
export interface PromotionSites {
  portal: string
  console: string
  api: string
  img: string
  service: string
}

/** 平台直营展示名 */
const PLATFORM_NAME = '星楼网络'
/** 平台直营英文副品牌 */
const PLATFORM_SUB_NAME = 'StarLoft'
/** 平台默认站点域名 */
const PLATFORM_SITES: PromotionSites = {
  portal: 'https://www.starloft.cn',
  console: 'https://console.starloft.cn',
  api: 'https://api.starloft.cn',
  img: 'https://img.starloft.cn',
  service: 'https://service.starloft.cn'
}

/** 站点展示品牌名：恒为平台品牌 */
export const siteName = computed<string>(() => PLATFORM_NAME)

/** 站点英文副品牌：恒为平台副品牌 */
export const siteSubName = computed<string>(() => PLATFORM_SUB_NAME)

/** 站点基地址（不含末尾斜杠）：恒为平台默认站点 */
export const siteBase = (kind: keyof PromotionSites): string => PLATFORM_SITES[kind].replace(/\/$/, '')

/** 控制台注册地址：恒为平台控制台注册页 */
export const registerUrl = computed<string>(() => `${siteBase('console')}/register`)

/** 控制台登录地址：恒为平台控制台登录页 */
export const loginUrl = computed<string>(() => `${siteBase('console')}/login`)