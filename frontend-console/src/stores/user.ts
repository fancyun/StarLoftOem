import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUserStore = defineStore('user', () => {
  // 用户端
  const token = ref(localStorage.getItem('token') || '')
  const userInfo = ref<any>(null)
  const isKycVerified = ref(false)

  const setToken = (newToken: string) => {
    token.value = newToken
    localStorage.setItem('token', newToken)
  }

  const setUserInfo = (info: any) => {
    userInfo.value = info
    // realname_status: 0-未实名 1-个人实名(kyc) 2-企业实名(kyb)
    isKycVerified.value = info.realname_status >= 1
  }

  const logout = () => {
    token.value = ''
    userInfo.value = null
    isKycVerified.value = false
    localStorage.removeItem('token')
  }

  const clearAuth = () => {
    logout()
  }

  return {
    token,
    userInfo,
    isKycVerified,
    setToken,
    setUserInfo,
    logout,
    clearAuth
  }
})
