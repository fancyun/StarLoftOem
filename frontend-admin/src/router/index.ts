import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAdminStore } from '@/stores/admin'

// 管理后台独立应用路由（部署于 admin.starloft.cn）
// 一级分区：/sys 平台管理、/sms 短信服务、/fv 人脸核验；各分区独立侧边栏，左上角切换
// 每个子路由标注 meta.permission：无对应权限的账号既看不到菜单，也无法直接输入 URL 进入
const routes: Array<RouteRecordRaw> = [
  {
    path: '/login',
    name: 'AdminLogin',
    component: () => import('@/views/admin/Login.vue')
  },
  {
    path: '/',
    redirect: '/sys/dashboard' // 兼容旧入口，实际落地页由路由守卫按权限决定
  },
  {
    path: '/sys',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    redirect: '/sys/dashboard', // 平台管理默认页
    children: [
      {
        path: 'dashboard',
        name: 'AdminDashboard',
        component: () => import('@/views/admin/Dashboard.vue'),
        meta: { permission: 'sys.dashboard' }
      },
      {
        path: 'users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/Users.vue'),
        meta: { permission: 'sys.users' }
      },
      {
        path: 'users/:id',
        name: 'AdminUserDetail',
        component: () => import('@/views/admin/UserDetail.vue'),
        meta: { permission: 'sys.users' }
      },
      {
        path: 'kyc',
        name: 'AdminKyc',
        component: () => import('@/views/admin/Kyc.vue'),
        meta: { permission: 'sys.users' }
      },
      {
        path: 'kyb',
        name: 'AdminKyb',
        component: () => import('@/views/admin/Kyb.vue'),
        meta: { permission: 'sys.users' }
      },
      {
        path: 'finance',
        name: 'AdminFinance',
        component: () => import('@/views/admin/Finance.vue'),
        meta: { permission: 'sys.finance' }
      },
      {
        path: 'bills',
        name: 'AdminBills',
        component: () => import('@/views/admin/Bills.vue'),
        meta: { permission: 'sys.finance' }
      },
      {
        path: 'payments',
        name: 'AdminPayments',
        component: () => import('@/views/admin/Payments.vue'),
        meta: { permission: 'sys.finance' }
      },
      {
        path: 'commissions',
        name: 'AdminCommissions',
        component: () => import('@/views/admin/Commissions.vue'),
        meta: { permission: 'sys.aff' }
      },
      {
        path: 'promotion/withdraws',
        name: 'AdminPromotionWithdraws',
        component: () => import('@/views/admin/PromotionWithdraws.vue'),
        meta: { permission: 'sys.aff' }
      },
      {
        path: 'sales',
        name: 'AdminSales',
        component: () => import('@/views/admin/Sales.vue'),
        meta: { permission: 'sys.sales' }
      },
      {
        path: 'admins',
        name: 'AdminAdmins',
        component: () => import('@/views/admin/Admins.vue'),
        meta: { permission: 'sys.admins' }
      },
      {
        path: 'settings',
        name: 'AdminSettings',
        component: () => import('@/views/admin/Settings.vue'),
        meta: { permission: 'sys.settings' }
      }
    ]
  },
  {
    path: '/sms',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    redirect: '/sms/stats', // 短信服务默认页
    children: [
      {
        path: 'stats',
        name: 'AdminSmsStats',
        component: () => import('@/views/admin/SMSStats.vue'),
        meta: { permission: 'sms.stats' }
      },
      {
        path: 'signs',
        name: 'AdminSmsSigns',
        component: () => import('@/views/admin/SmsSigns.vue'),
        meta: { permission: 'sms.signs' }
      },
      {
        path: 'templates',
        name: 'AdminSmsTemplates',
        component: () => import('@/views/admin/SmsTemplates.vue'),
        meta: { permission: 'sms.templates' }
      },
      {
        path: 'records',
        name: 'AdminSmsRecords',
        component: () => import('@/views/admin/SMSRecords.vue'),
        meta: { permission: 'sms.records' }
      },
      {
        path: 'replies',
        name: 'AdminSmsReplies',
        component: () => import('@/views/admin/SMSReplies.vue'),
        meta: { permission: 'sms.replies' }
      },
      {
        path: 'packs',
        name: 'AdminSmsPacks',
        component: () => import('@/views/admin/ResourcePacks.vue'),
        meta: { permission: 'sms.packs' }
      },
      {
        path: 'product-config',
        name: 'AdminSmsProductConfig',
        component: () => import('@/views/admin/ProductConfig.vue'),
        meta: { product: 'sms', permission: 'sms.product_config' }
      }
    ]
  },
  {
    path: '/fv',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    redirect: '/fv/stats', // 人脸核验默认页
    children: [
      {
        path: 'stats',
        name: 'AdminFvStats',
        component: () => import('@/views/admin/FVStats.vue'),
        meta: { permission: 'fv.stats' }
      },
      {
        path: 'records',
        name: 'AdminFvRecords',
        component: () => import('@/views/admin/FvRecords.vue'),
        meta: { permission: 'fv.records' }
      },
      {
        path: 'packs',
        name: 'AdminFvPacks',
        component: () => import('@/views/admin/ResourcePacks.vue'),
        meta: { permission: 'fv.packs' }
      },
      {
        path: 'product-config',
        name: 'AdminFvProductConfig',
        component: () => import('@/views/admin/ProductConfig.vue'),
        meta: { product: 'fv', permission: 'fv.product_config' }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫：登录态 + 权限校验（前端仅做体验，后端仍逐接口判权）
router.beforeEach(async (to, _from, next) => {
  const adminStore = useAdminStore()

  // 登录页：已登录且有可用落地页时直接进入
  if (to.path === '/login') {
    if (adminStore.adminToken && adminStore.landingPath) {
      next(adminStore.landingPath)
    } else {
      next()
    }
    return
  }

  if (!to.meta.requiresAdmin) {
    next()
    return
  }

  if (!adminStore.adminToken) {
    next('/login')
    return
  }

  // 登录态恢复：本地无账号信息时向服务端拉取一次（权限以服务端为准）
  if (!adminStore.adminInfo) {
    try {
      await adminStore.loadMe()
    } catch (error) {
      adminStore.clearAdminInfo()
      next('/login')
      return
    }
  }

  // 未分配任何权限：清登录态并提示，避免进入空白后台
  if (!adminStore.landingPath) {
    ElMessage.error('该账号未分配任何权限，请联系超级管理员')
    adminStore.clearAdminInfo()
    next('/login')
    return
  }

  const required = to.meta.permission as string | undefined
  if (required && !adminStore.has(required)) {
    next(adminStore.landingPath)
    return
  }

  next()
})

export default router