<template>
  <div class="admin-login-container">
    <div class="login-card">
      <h2 class="title">管理后台登录</h2>
      <el-form :model="loginForm" :rules="rules" ref="loginFormRef" class="login-form" @submit.prevent>
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="管理员用户名"
            prefix-icon="User"
            size="large"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="密码"
            prefix-icon="Lock"
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            native-type="button"
            size="large"
            class="login-btn"
            :loading="loading"
            :disabled="loading"
            @click.stop="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <div class="footer-tip">
        <span>非管理员请前往</span>
        <a href="https://console.starloft.cn" target="_blank" rel="noopener">用户控制台</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'

const router = useRouter()
const adminStore = useAdminStore()
const loginFormRef = ref()
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const handleLogin = async () => {
  if (loading.value) return
  
  try {
    await loginFormRef.value.validate()
  } catch (error) {
    return
  }
  
  loading.value = true
  
  try {
    const res: any = await adminAPI.login(loginForm)
    
    // 保存token和管理员信息（含权限清单，用于按权限渲染菜单与守卫）
    adminStore.setAdminToken(res.token)
    adminStore.setAdminInfo({
      admin_id: res.admin_id,
      username: res.username,
      nickname: res.nickname,
      permissions: res.permissions || [],
      is_super: !!res.is_super
    })

    ElMessage.success('登录成功')
    // 落地页按权限决定（无任何权限时守卫会拦截并提示）
    router.push(adminStore.landingPath || '/login')
  } catch (err: any) {
    ElMessage.error(err.response?.data?.message || err.message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(circle at 20% 0%, #E8F3FF 0%, transparent 55%),
    radial-gradient(circle at 100% 80%, #D6E8FF 0%, transparent 45%),
    var(--bg-page);
  padding: 20px;
}

.login-card {
  width: 420px;
  padding: 40px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}

.title {
  text-align: center;
  margin-bottom: 30px;
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.login-form {
  margin-top: 20px;
}

.login-btn {
  width: 100%;
  height: 42px;
  font-size: 15px;
}

.footer-tip {
  text-align: center;
  margin-top: 20px;
  font-size: 14px;
  color: var(--text-muted);
}

.footer-tip a {
  margin-left: 8px;
  text-decoration: none;
}
</style>
