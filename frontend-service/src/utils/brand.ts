/**
 * 品牌与站点域名：由后端公开配置 /console/config 的 brand 字段下发
 * （后台「系统设置 → 品牌与域名」维护），未取到时沿用本文件的默认值。
 * 各前端统一从此处取展示内容与站点基地址，避免品牌信息散落硬编码。
 */
import { reactive } from 'vue'

/** 站点域名组（不含末尾斜杠） */
export interface BrandSites {
  portal: string
  console: string
  api: string
  img: string
  service: string
}

/** 品牌信息 */
export interface BrandInfo {
  name: string
  sub_name: string
  icp: string
  company: string
  address: string
  copyright: string
  logo_url: string
  sites: BrandSites
}

/** 默认品牌（后端配置取不到时的兜底，与后端内置默认值保持一致） */
export const DEFAULT_BRAND: BrandInfo = {
  name: 'OEM 云服务',
  sub_name: '',
  icp: '',
  company: 'OEM 网络科技有限公司',
  address: '',
  copyright: '© OEM 云服务',
  logo_url: '',
  sites: {
    portal: 'https://www.oem.example.com',
    console: 'https://console.oem.example.com',
    api: 'https://api.oem.example.com',
    img: 'https://img.oem.example.com',
    service: 'https://service.oem.example.com'
  }
}

/** 当前品牌状态（响应式，配置加载后自动更新视图） */
export const brandState = reactive<BrandInfo>({
  ...DEFAULT_BRAND,
  sites: { ...DEFAULT_BRAND.sites }
})

let configPromise: Promise<any> | null = null

/** 拉取后端公开配置（幂等：全站共享同一次请求，失败静默返回 null） */
export function loadPublicConfig(): Promise<any> {
  if (!configPromise) {
    configPromise = fetch('/console/config')
      .then((res) => res.json())
      .catch(() => null)
  }
  return configPromise
}

/** 加载品牌配置（幂等） */
export function loadBrand(): Promise<BrandInfo> {
  return loadPublicConfig().then((json) => {
    const b = json?.data?.brand
    if (b && typeof b === 'object') {
      Object.assign(brandState, {
        name: b.name || DEFAULT_BRAND.name,
        sub_name: b.sub_name || '',
        icp: b.icp || '',
        company: b.company || DEFAULT_BRAND.company,
        address: b.address || '',
        copyright: b.copyright || DEFAULT_BRAND.copyright,
        logo_url: b.logo_url || '',
        sites: { ...DEFAULT_BRAND.sites, ...(b.sites || {}) }
      })
    }
    return brandState
  })
}

/** 取站点基地址（不含末尾斜杠） */
export function siteBase(kind: keyof BrandSites): string {
  return String(brandState.sites[kind] || '').replace(/\/$/, '')
}

/** 设置浏览器标签标题（品牌名 + 可选页面名） */
export function applyTitle(pageTitle?: string) {
  document.title = pageTitle ? `${brandState.name} · ${pageTitle}` : brandState.name
}