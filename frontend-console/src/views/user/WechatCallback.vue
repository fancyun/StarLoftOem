<template>
  <div class="wechat-page">
    <div class="card" v-loading="loading">
      <el-icon v-if="failed" class="state-icon"><CircleClose /></el-icon>
      <el-icon v-else class="state-icon success"><CircleCheck /></el-icon>
      <div class="state-title">{{ title }}</div>
      <div class="state-tip">{{ tip }}</div>
      <el-button v-if="failed" type="primary" @click="goLogin">返回登录</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(true)
const failed = ref(false)
const title = ref('正在登录')
const tip = ref('正在验证微信身份，请稍候...')

const goLogin = () => router.replace('/login')

onMounted(async () => {
  // 票据由后端授权回调 302 携带，一次性、两分钟内有效
  const ticket = String(route.query.ticket || '')
  if (!ticket) {
    loading.value = false
    failed.value = true
    title.value = '登录失败'
    tip.value = '登录票据缺失，请重新发起微信登录'
    return
  }

  try {
    const res: any = await userAPI.wechatTicket({ ticket })
    userStore.setToken(res.token)
    userStore.setUserInfo(res)
    title.value = '登录成功'
    tip.value = '正在进入控制台...'
    await router.replace('/dashboard')
  } catch (error: any) {
    failed.value = true
    title.value = '登录失败'
    tip.value = error?.message || '登录票据已失效，请重新发起微信登录'
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.wechat-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-page);
}

.card {
  width: 100%;
  max-width: 420px;
  padding: 40px 32px;
  text-align: center;
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.state-icon {
  font-size: 44px;
  color: var(--text-muted);
}

.state-icon.success {
  color: #07c160;
}

.state-title {
  margin-top: 16px;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.state-tip {
  margin: 8px 0 24px;
  font-size: 14px;
  color: var(--text-muted);
}
</style>