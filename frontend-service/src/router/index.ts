import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: Array<RouteRecordRaw> = [
  // FV 认证承接页（部署于 service.starloft.cn/fv/:mode，auth|self 两种模式）
  {
    path: '/fv/:mode',
    name: 'FaceVerify',
    component: () => import('@/views/fv/FaceVerify.vue'),
    props: true
  },
  // 未匹配路由默认落到 auth 模式的承接页
  {
    path: '/:pathMatch(.*)*',
    redirect: '/fv/auth'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router