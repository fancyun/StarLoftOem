import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/stores/user'

// 控制台应用路由（部署于 console.starloft.cn）
// 产品化结构：人脸核验 /fv、短信服务 /sms 均为顶层产品路径。
// 账户实名页为 /certification；原 /kyc 保留为兼容别名。
const routes: Array<RouteRecordRaw> = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/user/Login.vue')
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/user/Register.vue')
  },
  // 微信一键登录中转页（公开路由）：微信授权回跳后凭一次性票据换取登录态；
  // 路径不能以 /console 开头（该前缀会被 Nginx 反代到后端，SPA 不渲染）
  {
    path: '/wechat/callback',
    name: 'WechatCallback',
    component: () => import('@/views/user/WechatCallback.vue')
  },
  // 微信未绑定时的绑定已有账号页（公开路由）
  {
    path: '/wechat/bind',
    name: 'WechatBind',
    component: () => import('@/views/user/WechatBind.vue')
  },
  // 控制台（需登录）
  {
    path: '/',
    component: () => import('@/views/user/UserLayout.vue'),
    meta: { requiresAuth: true },
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/user/Dashboard.vue')
      },
      {
        path: 'certification',
        name: 'Certification',
        component: () => import('@/views/user/KYC.vue')
      },
      // ===== 产品页面（侧边栏按一级分区渲染菜单，页面直接挂载于控制台布局） =====
      // 人脸核验 fv：认证记录 / 在线调用 / 资源包（实名认证为账户级，位于「用户中心」）
      { path: 'fv', redirect: '/fv/records' },
      { path: 'fv/kyc', redirect: '/certification' },
      { path: 'fv/records', name: 'FVRecords', component: () => import('@/views/user/Records.vue') },
      { path: 'fv/invoke', name: 'FVInvoke', component: () => import('@/views/user/FVInvoke.vue') },
      { path: 'fv/packs', name: 'FVPacks', component: () => import('@/views/user/ResourcePacks.vue') },
      { path: 'fv/packs/buy', name: 'FVPurchasePacks', component: () => import('@/views/user/PurchasePacks.vue') },
      // 短信服务 sms：签名 / 模板 / 资源包 / 发送记录 / 统计
      { path: 'sms', redirect: '/sms/sign' },
      { path: 'sms/sign', name: 'SMSSign', component: () => import('@/views/user/SMSSign.vue') },
      // 提交/修改签名、模板均为独立页面（列表页只做管理与查询）
      { path: 'sms/sign/create', name: 'SMSSignCreate', component: () => import('@/views/user/SMSSignForm.vue') },
      { path: 'sms/sign/edit/:id', name: 'SMSSignEdit', component: () => import('@/views/user/SMSSignForm.vue') },
      { path: 'sms/template', name: 'SMSTemplate', component: () => import('@/views/user/SMSTemplate.vue') },
      { path: 'sms/template/create', name: 'SMSTemplateCreate', component: () => import('@/views/user/SMSTemplateForm.vue') },
      { path: 'sms/template/edit/:id', name: 'SMSTemplateEdit', component: () => import('@/views/user/SMSTemplateForm.vue') },
      { path: 'sms/send', name: 'SMSSend', component: () => import('@/views/user/SMSSend.vue') },
      { path: 'sms/packs', name: 'SMSPacks', component: () => import('@/views/user/SMSResourcePacks.vue') },
      { path: 'sms/packs/buy', name: 'SMSPurchasePacks', component: () => import('@/views/user/SMSPurchasePacks.vue') },
      { path: 'sms/records', name: 'SMSRecords', component: () => import('@/views/user/SMSRecords.vue') },
      { path: 'sms/stats', name: 'SMSStats', component: () => import('@/views/user/SMSStats.vue') },
      // 推广 promotion：推广链接 / 收益列表 / 推广用户（单页，顶部按钮切换）
      { path: 'promotion', name: 'Promotion', component: () => import('@/views/user/Promotion.vue') },
      // 兼容别名：原 /kyc 及子路径
      {
        path: 'kyc',
        name: 'KYC',
        component: () => import('@/views/user/KYC.vue')
      },
      {
        path: 'kyc/records',
        name: 'RecordsCompat',
        component: () => import('@/views/user/Records.vue')
      },
      {
        path: 'kyc/packs',
        name: 'ResourcePacksCompat',
        component: () => import('@/views/user/ResourcePacks.vue')
      },
      {
        path: 'kyc/packs/buy',
        name: 'PurchasePacksCompat',
        component: () => import('@/views/user/PurchasePacks.vue')
      },
      {
        path: 'balance',
        name: 'Balance',
        component: () => import('@/views/user/Balance.vue')
      },
      {
        path: 'payment/:pay_order_no',
        name: 'PayDetail',
        component: () => import('@/views/user/PayDetail.vue')
      },
      {
        path: 'settings',
        name: 'AccountSettings',
        component: () => import('@/views/user/AccountSettings.vue')
      },
      {
        path: 'api',
        name: 'APIManagement',
        component: () => import('@/views/user/APIManagement.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, _from, next) => {
  const userStore = useUserStore()
  const requiresAuth = to.meta.requiresAuth

  if (requiresAuth && !userStore.token) {
    // 普通用户页面需要登录但未登录
    next('/login')
  } else if (!requiresAuth && userStore.token && (to.path === '/login' || to.path === '/register')) {
    // 普通用户已登录访问登录/注册页，跳转到控制台首页
    next('/dashboard')
  } else {
    next()
  }
})

export default router
