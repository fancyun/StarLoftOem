import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { adminAPI } from '@/api'
import { LANDING_ORDER, PERMISSION_ALL, WRITE_SUFFIX, isGroupWildcard } from '@/permissions'

export const useAdminStore = defineStore('admin', () => {
  // 管理员端
  const adminToken = ref(localStorage.getItem('admin_token') || '')
  // 账号信息（含权限清单）本地持久化，刷新后无需再次请求即可重建菜单
  const adminInfo = ref<any>(JSON.parse(localStorage.getItem('admin_info') || 'null'))

  const setAdminToken = (newToken: string) => {
    adminToken.value = newToken
    localStorage.setItem('admin_token', newToken)
  }

  const setAdminInfo = (info: any) => {
    adminInfo.value = info
    if (info) {
      localStorage.setItem('admin_info', JSON.stringify(info))
    } else {
      localStorage.removeItem('admin_info')
    }
  }

  const clearAdminInfo = () => {
    adminToken.value = ''
    adminInfo.value = null
    localStorage.removeItem('admin_token')
    localStorage.removeItem('admin_info')
  }

  // 权限清单（all / 分组通配码 sys·sms·fv / 读码 / 写码 按原样存储）
  const adminPermissions = computed<string[]>(() => adminInfo.value?.permissions || [])
  const isSuper = computed(() => adminPermissions.value.includes(PERMISSION_ALL) || !!adminInfo.value?.is_super)
  // has 是否持有指定权限码（与后端 config.HasPermission 同规则）：
  // all 覆盖一切；分组通配码覆盖同前缀的读/写码；写权限码隐含同模块读权限。
  const has = (code: string) =>
    isSuper.value ||
    adminPermissions.value.some(
      (token) =>
        token === code ||
        (isGroupWildcard(token) && code.startsWith(token + '.')) ||
        token === code + WRITE_SUFFIX
    )
  // 登录后落地页：第一个有权限的页面；无任何权限时为空串
  const landingPath = computed(() => LANDING_ORDER.find((item) => has(item.code))?.path || '')

  // loadMe 拉取当前账号信息（权限以服务端为准，登录态恢复与权限校验时调用）
  const loadMe = async () => {
    const me: any = await adminAPI.me()
    setAdminInfo(me)
    return me
  }

  return {
    adminToken,
    adminInfo,
    adminPermissions,
    isSuper,
    has,
    landingPath,
    loadMe,
    setAdminToken,
    setAdminInfo,
    clearAdminInfo
  }
})