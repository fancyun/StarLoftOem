import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

// 门户应用路由（部署于 www.starloft.cn）
// 产品页均为一级（URL 用产品简称）：
//   /product/fv    人脸核验产品页
//   /product/sms   短信服务产品页
// 产品页共用 ProductPage.vue，通过 meta.entryKey 指定入口 key。
// 文档中心按层级：/docs（文档中心首页）→ /docs/{产品}（产品介绍、api、plugin、pricing；
//   三级/四级目录如 api 分版本与接口、plugin 分适用系统与插件类型）。

// 各产品插件文档（三级=适用系统，四级=插件类型）
const pluginDocs = (product: string): Array<RouteRecordRaw> => {
  if (product === 'fv') {
    return [
      { path: 'plugin/star_loft_fv_for_zjmf_mfcw', name: 'StarLoftFvForZjmfMfcwPlugin', component: () => import('@/views/docs/PluginZjmfMfcwFv.vue') },
      { path: 'plugin/star_loft_fv_for_zjmf_v10', name: 'StarLoftFvForZjmfV10', component: () => import('@/views/docs/PluginZjmfV10Fv.vue') }
    ]
  }
  return []
}

// 单产品文档路由：/docs/{产品} 下挂 docs/api/api/v1/pricing/plugin
const productDocs = (product: string): RouteRecordRaw => ({
  path: `/docs/${product}`,
  component: () => import('@/views/docs/DocsLayout.vue'),
  children: [
    { path: '', redirect: `/docs/${product}/docs` },
    { path: 'docs', name: `${product}DocsHome`, component: () => import('@/views/docs/DocsHome.vue') },
    { path: 'api', name: `${product}ApiDocs`, component: () => import('@/views/docs/ApiDocs.vue') },
    { path: 'api/v1', name: `${product}ApiV1`, component: () => import('@/views/docs/ApiV1.vue') },
    { path: 'pricing', name: `${product}Pricing`, component: () => import('@/views/docs/Pricing.vue') },
    { path: 'plugin', name: `${product}PluginDocs`, component: () => import('@/views/docs/PluginDocs.vue') },
    ...pluginDocs(product)
  ]
})
const routes: Array<RouteRecordRaw> = [
  {
    path: '/',
    component: () => import('@/layouts/SiteLayout.vue'),
    children: [
      {
        path: '',
        name: 'Home',
        component: () => import('@/views/Home.vue')
      },
      {
        path: 'product/fv',
        name: 'FaceVerification',
        component: () => import('@/views/ProductPage.vue'),
        meta: { entryKey: 'fv' }
      },
      {
        path: 'product/sms',
        name: 'SMS',
        component: () => import('@/views/ProductPage.vue'),
        meta: { entryKey: 'sms' }
      },
      {
        path: 'terms',
        name: 'Terms',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'terms', title: '用户协议' }
      },
      {
        path: 'privacy',
        name: 'Privacy',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'privacy', title: '隐私政策' }
      },
      {
        path: 'sms-rules',
        name: 'SmsRules',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'sms', title: '短信服务条款' }
      },
      {
        path: 'auth-authorization',
        name: 'AuthAuthorization',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'auth', title: '实名认证授权协议' }
      },
      {
        path: 'sla',
        name: 'SLA',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'sla', title: '服务等级协议（SLA）' }
      },
      {
        path: 'billing',
        name: 'Billing',
        component: () => import('@/views/Agreement.vue'),
        meta: { docType: 'billing', title: '费用与退款说明' }
      }
    ]
  },
  // 文档中心首页：/docs，进入产品后二级为 docs/api/plugin/pricing
  {
    path: '/docs',
    name: 'DocsCenter',
    component: () => import('@/views/docs/DocsCenter.vue')
  },
  // 文档中心（按产品分层：/docs/{产品} 各挂 docs/api/api/v1/plugin/pricing）
  productDocs('fv'),
  productDocs('sms'),
  // 未匹配路由回退首页
  {
    path: '/:pathMatch(.*)*',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  }
})

export default router