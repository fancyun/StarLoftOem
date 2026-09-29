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
          <h1>创建您的账户<br/>开启云上之旅</h1>
          <p>注册即享完整的云产品服务，快速接入，安全可靠</p>
        </div>
        <div class="banner-steps">
          <div class="step-item">
            <span class="step-num">01</span>
            <div class="step-info">
              <strong>填写信息</strong>
              <span>手机号快速注册</span>
            </div>
          </div>
          <div class="step-item">
            <span class="step-num">02</span>
            <div class="step-info">
              <strong>账户充值</strong>
              <span>选择套餐立即充值</span>
            </div>
          </div>
          <div class="step-item">
            <span class="step-num">03</span>
            <div class="step-info">
              <strong>开始使用</strong>
              <span>按需调用云产品</span>
            </div>
          </div>
        </div>
      </div>
      <div class="banner-bg-pattern"></div>
    </div>

    <!-- 右侧表单区 -->
    <div class="auth-form-panel">
      <div class="form-wrapper">
        <div class="form-header">
          <div v-if="promotionName" class="promotion-brand">您正在注册 {{ promotionName }} 的账号</div>
          <h2>创建账号</h2>
          <p>填写以下信息完成注册</p>
        </div>

        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form" @submit.prevent="handleRegister">
          <el-form-item prop="username">
            <el-input
              v-model="form.username"
              placeholder="请输入用户名（英文/数字/下划线）"
              size="large"
              prefix-icon="User"
              clearable
              maxlength="32"
            />
          </el-form-item>

          <el-form-item prop="phone">
            <el-input
              v-model="form.phone"
              placeholder="请输入手机号"
              size="large"
              prefix-icon="Phone"
              clearable
            />
          </el-form-item>

          <el-form-item prop="sms_code">
            <div class="sms-row">
              <el-input
                v-model="form.sms_code"
                placeholder="请输入手机验证码"
                size="large"
                prefix-icon="Message"
              />
              <el-button
                size="large"
                :disabled="smsCountdown > 0 || !form.phone"
                class="sms-btn"
                @click="sendSMSCode"
              >
                {{ smsCountdown > 0 ? `${smsCountdown}s` : '获取验证码' }}
              </el-button>
            </div>
          </el-form-item>

          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="请设置密码（8-32位，含字母和数字）"
              size="large"
              prefix-icon="Lock"
              show-password
            />
          </el-form-item>

          <el-form-item prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="请再次输入密码"
              size="large"
              prefix-icon="Lock"
              show-password
            />
          </el-form-item>

          <el-form-item prop="agree">
            <el-checkbox v-model="form.agree">
              <span class="agree-text">
                我已阅读并同意
                <a :href="`${siteBase('portal')}/terms`" target="_blank" rel="noopener" class="agree-link">《用户协议》</a>和
                <a :href="`${siteBase('portal')}/privacy`" target="_blank" rel="noopener" class="agree-link">《隐私政策》</a>
              </span>
            </el-checkbox>
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
              {{ loading ? '注册中...' : '注 册' }}
            </el-button>
          </el-form-item>
        </el-form>

        <div class="form-footer">
          <span class="footer-text">已有账号？</span>
          <router-link to="/login" class="footer-link">立即登录</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { verifyCaptcha } from '@/utils/captcha'
import { resolvePromotion, promotionSourceDomain, siteBase, siteState } from '@/utils/promotion'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const loading = ref(false)
const smsCountdown = ref(0)
const formRef = ref()
// 归属推广商：优先推广链接的 ?ref= 推广码，无码时回退按当前站点域名判定（均由后端解析绑定）
const promotionDomain = ref(promotionSourceDomain())
const promotionName = ref('')
const affRef = ref(String(route.query.ref || '').trim())
// 品牌区标题/副标题：命中推广品牌时用其品牌名并隐藏平台副品牌
const brandTitle = computed(() => (siteState.matched && siteState.name ? siteState.name : 'StarLoft'))
const brandSub = computed(() => (siteState.matched ? '' : 'Cloud Services'))

const form = reactive({
  username: '',
  phone: '',
  sms_code: '',
  password: '',
  confirmPassword: '',
  agree: false
})

const validatePass = (_rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== form.password) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]{3,32}$/, message: '用户名仅支持英文、数字、下划线，长度3-32位', trigger: 'blur' }
  ],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' }
  ],
  sms_code: [{ required: true, message: '请输入手机验证码', trigger: 'blur' }],
  password: [
    { required: true, message: '请设置密码', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d)[^\s]{8,32}$/, message: '密码需8-32位且同时包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [{ validator: validatePass, trigger: 'blur' }],
  agree: [
    {
      validator: (_rule: any, value: any, callback: any) => {
        if (!value) callback(new Error('请阅读并同意用户协议'))
        else callback()
      },
      trigger: 'change'
    }
  ]
}

const getErrorMessage = (error: any) => error?.response?.data?.message || error?.message || '操作失败'

const sendSMSCode = async () => {
  if (!form.phone) {
    ElMessage.warning('请输入手机号')
    return
  }
  if (smsCountdown.value > 0) return
  try {
    const { ticket, randstr } = await verifyCaptcha()
    await userAPI.sendSMSCode({
      phone: form.phone,
      captcha_ticket: ticket,
      captcha_randstr: randstr,
      scene: 'register'
    })
    ElMessage.success('验证码已发送')
    smsCountdown.value = 60
    const timer = setInterval(() => {
      smsCountdown.value--
      if (smsCountdown.value <= 0) clearInterval(timer)
    }, 1000)
  } catch (error: any) {
    const msg = getErrorMessage(error)
    if (msg !== '用户取消验证') ElMessage.error(msg)
  }
}

const handleRegister = async () => {
  if (loading.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    const { ticket, randstr } = await verifyCaptcha()
    const result = await userAPI.register({
      username: form.username,
      phone: form.phone,
      sms_code: form.sms_code,
      password: form.password,
      captcha_ticket: ticket,
      captcha_randstr: randstr,
      domain: promotionDomain.value,
      ref: affRef.value
    })

    // 注册成功自动登录
    if (result.token) {
      userStore.setToken(result.token)
      userStore.setUserInfo(result)
      ElMessage.success('注册成功')
      router.push('/dashboard')
    } else {
      ElMessage.success('注册成功，请登录')
      router.push('/login')
    }
  } catch (error: any) {
    const msg = getErrorMessage(error)
    if (msg !== '用户取消验证') ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

const loadPromotion = async () => {
  const promotion = await resolvePromotion()
  if (promotion) {
    promotionName.value = promotion.name
  }
}

onMounted(loadPromotion)
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
    radial-gradient(circle at 80% 20%, rgba(0,110,255,0.06) 0%, transparent 50%),
    radial-gradient(circle at 20% 80%, rgba(0,110,255,0.04) 0%, transparent 50%);
  pointer-events: none;
}

.banner-bg-pattern::before {
  content: '';
  position: absolute;
  top: -40%;
  left: -20%;
  width: 500px;
  height: 500px;
  border-radius: 50%;
  border: 1px solid rgba(0,110,255,0.08);
}

.banner-bg-pattern::after {
  content: '';
  position: absolute;
  bottom: -30%;
  right: -10%;
  width: 350px;
  height: 350px;
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

/* 注册步骤 */
.banner-steps {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 14px;
}

.step-num {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: rgba(0,110,255,0.1);
  font-size: 13px;
  font-weight: 700;
  color: var(--color-primary);
  flex-shrink: 0;
}

.step-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.step-info strong {
  font-size: 14px;
  color: var(--text-primary);
  font-weight: 600;
}

.step-info span {
  font-size: 12px;
  color: var(--text-muted);
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
  transition: box-shadow 0.2s;
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

/* 协议复选框 */
.agree-text {
  font-size: 13px;
  color: var(--text-muted);
}

.agree-link {
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 500;
}

.agree-link:hover {
  text-decoration: underline;
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