// 站点品牌与站点域名：全部由后端公开配置下发（后台「系统设置 → 品牌与域名」维护），
// 未取到时回落 brand.ts 的默认值；各页面统一从此处取展示内容，不再硬编码品牌。

import { computed } from 'vue'

import { brandState, siteBase as brandSiteBase, type BrandSites } from './brand'

/** 站点域名组 */
export type PromotionSites = BrandSites

/** 站点展示品牌名 */
export const siteName = computed<string>(() => brandState.name)

/** 站点副品牌/副标题 */
export const siteSubName = computed<string>(() => brandState.sub_name)

/** 站点基地址（不含末尾斜杠） */
export const siteBase = (kind: keyof PromotionSites): string => brandSiteBase(kind)

/** 控制台注册地址 */
export const registerUrl = computed<string>(() => `${siteBase('console')}/register`)

/** 控制台登录地址 */
export const loginUrl = computed<string>(() => `${siteBase('console')}/login`)