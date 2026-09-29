<template>
  <div class="auth-container">
    <!-- 左侧品牌展示区 -->
    <div class="auth-banner">
      <div class="banner-content">
        <div class="banner-logo">
          <img class="brand-logo lg" src="/logo.jpg" :alt="brandTitle" />
          <div class="logo-text">
            <h2>{{ brandTitle }}</h2>
            <span v-if="brandSub">{{ brandSub }}</span>
          </div>
        </div>
        <div class="banner-desc">
          <h1>安全可靠的<br/>云服务平台</h1>
          <p>简单接入、稳定可靠，为您的业务保驾护航</p>
        </div>
        <div class="banner-features">
          <div class="feature-item">
            <span class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
            </span>
            <span>银行级数据加密</span>
          </div>
          <div class="feature-item">
            <span class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
            </span>
            <span>99.9% 服务可用性</span>
          </div>
          <div class="feature-item">
            <span class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </span>
            <span>秒级响应速度</span>
          </div>
        </div>
      </div>
      <div class="banner-bg-pattern"></div>
    </div>

    <!-- 右侧表单区 -->
    <div class="auth-form-panel">
      <div class="form-wrapper">
        <div class="form-header">
          <div v-if="promotionName" class="promotion-brand">您正在登录 {{ promotionName }} 的账号</div>
          <h2>欢迎回来</h2>
          <p>登录您的账户以继续使用</p>
        </div>

        <el-tabs v-model="activeTab" class="auth-tabs">
          <el-tab-pane label="密码登录" name="password">
            <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" class="auth-form" @submit.prevent="handlePasswordLogin">
              <el-form-item prop="account">
                <el-input
                  v-model="passwordForm.account"
                  placeholder="请输入用户名 / 手机号"
                  size="large"
                  :prefix-icon="UserIcon"
                  clearable
                />
              </el-form-item>
              <el-form-item prop="password">
                <el-input
                  v-model="passwordForm.password"
                  type="password"
                  placeholder="请输入密码"
                  size="large"
                  :prefix-icon="LockIcon"
                  show-password
                  @keyup.enter="handlePasswordLogin"
                />
              </el-form-item>
              <div class="form-extra">
                <span class="form-link" @click="activeTab = 'sms'">忘记密码？验证码登录</span>
              </div>
              <el-form-item>
                <el-button
                  type="primary"
                  native-type="submit"
                  size="large"
                  class="submit-btn"
                  :loading="loading"
                  :disabled="loading"
                >
                  {{ loading ? '登录中...' : '登 录' }}
                </el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>

          <el-tab-pane label="验证码登录" name="sms">
            <el-form ref="smsFormRef" :model="smsForm" :rules="rules" class="auth-form" @submit.prevent="handleSMSLogin">
              <el-form-item prop="phone">
                <el-input
                  v-model="smsForm.phone"
                  placeholder="请输入手机号"
                  size="large"
                  :prefix-icon="PhoneIcon"
                  clearable
                />
              </el-form-item>
              <el-form-item prop="sms_code">
                <div class="sms-row">
                  <el-input
                    v-model="smsForm.sms_code"
                    placeholder="请输入验证码"
                    size="large"
                    :prefix-icon="MessageIcon"
                    @keyup.enter="handleSMSLogin"
                  />
                  <el-button
                    size="large"
                    :disabled="countdown > 0 || !smsForm.phone"
                    class="sms-btn"
                    @click="sendCode"
                  >
                    {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
                  </el-button>
                </div>
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  native-type="submit"
                  size="large"
                  class="submit-btn"
                  :loading="loading"
                  :disabled="loading"
                >
                  {{ loading ? '登录中...' : '登 录' }}
                </el-button>
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>

        <div class="form-footer">
          <span class="footer-text">还没有账号？</span>
          <router-link to="/register" class="footer-link">立即注册</router-link>
        </div>
        <div class="form-legal">
          <a :href="`${siteBase('portal')}/terms`" target="_blank" rel="noopener" class="legal-link">《用户协议》</a>
          <a :href="`${siteBase('portal')}/privacy`" target="_blank" rel="noopener" class="legal-link">《隐私政策》</a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, shallowRef, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { resetCaptchaCache, verifyCaptcha } from '@/utils/captcha'
import { resolvePromotion, siteBase, siteState } from '@/utils/promotion'
import { brandState } from '@/utils/brand'

const router = useRouter()
const userStore = useUserStore()
const activeTab = ref('password')
const loading = ref(false)
const countdown = ref(0)
const passwordFormRef = ref<FormInstance>()
const smsFormRef = ref<FormInstance>()
// 命中推广品牌时展示其品牌名
const promotionName = ref('')
// 品牌区标题/副标题：均取自后端下发的品牌配置（后台「系统设置 → 品牌与域名」维护）
const brandTitle = computed(() => siteState.name || brandState.name)
const brandSub = computed(() => brandState.sub_name)

const passwordForm = reactive({ account: '', password: '' })
const smsForm = reactive({ phone: '', sms_code: '' })

// 密码登录：账号支持用户名/手机号
const passwordRules = {
  account: [{ required: true, message: '请输入用户名/手机号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// 验证码登录：仅支持手机号
const rules = {
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' }
  ],
  sms_code: [{ required: true, message: '请输入验证码', trigger: 'blur' }]
}

// SVG 图标组件（避免 element-plus icon 组件名与字符串不一致）
const UserIcon = shallowRef('User')
const LockIcon = shallowRef('Lock')
const MessageIcon = shallowRef('Message')
const PhoneIcon = shallowRef('Phone')

resetCaptchaCache()

const getErrorMessage = (error: any) => error?.response?.data?.message || error?.message || '操作失败'

const login = async (payload: { account?: string; password?: string; sms_code?: string; login_type: 'password' | 'sms_code' }) => {
  const captcha = await verifyCaptcha()
  const result: any = await userAPI.login({ ...payload, captcha_payload: captcha.payload })
  if (!result || !result.token) {
    throw new Error('登录响应数据异常，请检查后端服务是否已更新')
  }
  userStore.setToken(result.token)
  userStore.setUserInfo(result)
  ElMessage.success('登录成功')
  await router.push('/dashboard')
}

const handlePasswordLogin = async () => {
  if (loading.value) return
  try {
    await passwordFormRef.value?.validate()
    loading.value = true
    await login({ account: passwordForm.account, password: passwordForm.password, login_type: 'password' })
  } catch (error: any) {
    const msg = getErrorMessage(error)
    if (msg !== '用户取消验证') ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

const handleSMSLogin = async () => {
  if (loading.value) return
  try {
    await smsFormRef.value?.validate()
    loading.value = true
    await login({ account: smsForm.phone, sms_code: smsForm.sms_code, login_type: 'sms_code' })
  } catch (error: any) {
    const msg = getErrorMessage(error)
    if (msg !== '用户取消验证') ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

const sendCode = async () => {
  if (!smsForm.phone) {
    ElMessage.warning('请输入手机号')
    return
  }
  if (countdown.value > 0) return
  try {
    const captcha = await verifyCaptcha()
    await userAPI.sendSMSCode({ phone: smsForm.phone, captcha_payload: captcha.payload, scene: 'login' })
    ElMessage.success('验证码已发送')
    countdown.value = 60
    const timer = window.setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0) window.clearInterval(timer)
    }, 1000)
  } catch (error: any) {
    const msg = getErrorMessage(error)
    if (msg !== '用户取消验证') ElMessage.error(msg)
  }
}

onMounted(async () => {
  const promotion = await resolvePromotion()
  if (promotion) promotionName.value = promotion.name
})
</script>

<style scoped>
/* ========== 整体布局 ========== */
.auth-container {
  display: flex;
  min-height: 100vh;
  background: var(--bg-page);
}

/* ========== 左侧品牌区（浅色腾讯云风格） ========== */
.auth-banner {
  flex: 1;
  position: relative;
  background: linear-gradient(135deg, #E8F3FF 0%, #DCEBFF 50%, #C2DEFF 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 60px;
}

.banner-bg-pattern {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 20% 30%, rgba(0,110,255,0.06) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(0,110,255,0.04) 0%, transparent 50%);
  pointer-events: none;
}

.banner-bg-pattern::before {
  content: '';
  position: absolute;
  top: -50%;
  right: -30%;
  width: 600px;
  height: 600px;
  border-radius: 50%;
  border: 1px solid rgba(0,110,255,0.08);
}

.banner-bg-pattern::after {
  content: '';
  position: absolute;
  bottom: -20%;
  left: -10%;
  width: 400px;
  height: 400px;
  border-radius: 50%;
  border: 1px solid rgba(0,110,255,0.06);
}

.banner-content {
  position: relative;
  z-index: 1;
  max-width: 440px;
  color: var(--text-primary);
}

.banner-logo {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 48px;
}

.logo-text h2 {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.5px;
}

.logo-text span {
  font-size: 11px;
  color: var(--text-muted);
  letter-spacing: 2px;
  text-transform: uppercase;
}

.banner-desc h1 {
  font-size: 32px;
  font-weight: 700;
  color: var(--color-primary-active);
  line-height: 1.3;
  margin-bottom: 16px;
}

.banner-desc p {
  font-size: 15px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin-bottom: 48px;
}

.banner-features {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.feature-item {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
  color: var(--text-secondary);
}

.feature-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: rgba(0,110,255,0.08);
  color: var(--color-primary);
  flex-shrink: 0;
}

/* ========== 右侧表单区 ========== */
.auth-form-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: var(--bg-card);
}

.form-wrapper {
  width: 100%;
  max-width: 420px;
}

.form-header {
  text-align: center;
  margin-bottom: 32px;
}

.promotion-brand {
  display: inline-block;
  margin-bottom: 12px;
  padding: 4px 12px;
  border-radius: var(--radius-sm);
  background: var(--color-primary-light);
  color: var(--color-primary);
  font-size: 13px;
  font-weight: 500;
}

.form-header h2 {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.form-header p {
  font-size: 14px;
  color: var(--text-muted);
}

/* ========== Tabs 样式 ========== */
.auth-tabs {
  margin-bottom: 8px;
}

:deep(.auth-tabs .el-tabs__header) {
  margin-bottom: 24px;
}

:deep(.auth-tabs .el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--border-light);
}

:deep(.auth-tabs .el-tabs__item) {
  font-size: 15px;
  font-weight: 500;
  height: 44px;
  line-height: 44px;
  color: var(--text-muted);
  padding: 0 20px;
}

:deep(.auth-tabs .el-tabs__item.is-active) {
  color: var(--color-primary);
  font-weight: 600;
}

:deep(.auth-tabs .el-tabs__active-bar) {
  height: 2px;
  border-radius: 1px;
}

/* ========== 表单样式 ========== */
.auth-form {
  margin-top: 4px;
}

:deep(.auth-form .el-form-item) {
  margin-bottom: 20px;
}

:deep(.auth-form .el-input__wrapper) {
  border-radius: 6px;
  box-shadow: 0 0 0 1px var(--border-color) inset;
  padding: 0 16px;
  transition: box-shadow 0.2s, border-color 0.2s;
}

:deep(.auth-form .el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #C9CDD4 inset;
}

:deep(.auth-form .el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 2px var(--color-primary-light) inset, 0 0 0 1px var(--color-primary) inset;
}

:deep(.auth-form .el-input__inner) {
  height: 46px;
  line-height: 46px;
}

.form-extra {
  display: flex;
  justify-content: flex-end;
  margin-top: -12px;
  margin-bottom: 4px;
}

.form-link {
  font-size: 13px;
  color: var(--text-muted);
  text-decoration: none;
  transition: color 0.2s;
}

.form-link:hover {
  color: var(--color-primary);
}

/* 验证码行：按钮随输入框等高，固定宽度与输入框同尺寸对齐 */
.sms-row {
  display: flex;
  width: 100%;
  align-items: stretch;
  gap: 10px;
}

.sms-row .el-input {
  flex: 1;
  min-width: 0;
}

.sms-btn {
  flex-shrink: 0;
  box-sizing: border-box;
  width: 128px;
  height: 46px;
  padding: 0;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

/* 提交按钮 */
.submit-btn {
  width: 100%;
  height: 48px;
  border-radius: 6px;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 2px;
  margin-top: 4px;
  transition: all 0.3s;
}

.submit-btn:not(:disabled):hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(0, 110, 255, 0.35);
}

/* ========== 底部链接 ========== */
.form-footer {
  text-align: center;
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--border-light);
}

.footer-text {
  font-size: 14px;
  color: var(--text-muted);
}

.footer-link {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-primary);
  text-decoration: none;
  margin-left: 4px;
  transition: color 0.2s;
}

.footer-link:hover {
  color: var(--color-primary-hover);
}

/* 登录页底部协议链接 */
.form-legal {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-top: 12px;
}

.legal-link {
  font-size: 12px;
  color: var(--text-muted);
  text-decoration: none;
  transition: color 0.2s;
}

.legal-link:hover {
  color: var(--color-primary);
}

/* ========== 响应式 ========== */
@media (max-width: 768px) {
  .auth-banner {
    display: none;
  }

  .auth-form-panel {
    padding: 30px 20px;
  }

  .form-wrapper {
    max-width: 100%;
  }
}
</style>