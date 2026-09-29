<template>
  <div class="wechat-page">
    <div class="card">
      <h3 class="card-title">绑定已有账号</h3>
      <p class="card-tip">
        该微信尚未绑定平台账号。请用平台注册手机号验证身份完成绑定，绑定后即可用微信一键登录。
      </p>

      <el-form :model="form" label-width="90px">
        <el-form-item label="手机号">
          <el-input v-model="form.phone" placeholder="请输入平台注册手机号" clearable />
        </el-form-item>
        <el-form-item label="短信验证码">
          <div class="sms-row">
            <el-input v-model="form.sms_code" placeholder="请输入短信验证码" />
            <el-button :disabled="countdown > 0 || !form.phone" class="sms-btn" @click="sendCode">
              {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="submitting" @click="handleBind">绑定并登录</el-button>
          <el-button @click="goRegister">去注册</el-button>
        </el-form-item>
      </el-form>
      <p class="card-note">还没有平台账号？注册后请重新发起微信登录完成绑定。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { verifyCaptcha } from '@/utils/captcha'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const submitting = ref(false)
const countdown = ref(0)

const form = reactive({ phone: '', sms_code: '' })

const bindTicket = String(route.query.ticket || '')

const goRegister = () => router.replace('/register')

const sendCode = async () => {
  if (!/^1[3-9]\d{9}$/.test(form.phone)) {
    ElMessage.warning('请输入正确的手机号')
    return
  }
  if (countdown.value > 0) return
  try {
    const captcha = await verifyCaptcha()
    await userAPI.sendSMSCode({
      phone: form.phone,
      captcha_payload: captcha.payload,
      scene: 'login'
    })
    ElMessage.success('验证码已发送')
    countdown.value = 60
    const timer = window.setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0) window.clearInterval(timer)
    }, 1000)
  } catch (error: any) {
    if (error?.message !== '用户取消验证') ElMessage.error(error?.message || '验证码发送失败')
  }
}

const handleBind = async () => {
  if (!bindTicket) {
    ElMessage.error('绑定信息已失效，请重新发起微信登录')
    return
  }
  if (!/^1[3-9]\d{9}$/.test(form.phone) || !form.sms_code) {
    ElMessage.warning('请填写手机号与短信验证码')
    return
  }
  submitting.value = true
  try {
    const res: any = await userAPI.wechatBind({
      bind_ticket: bindTicket,
      phone: form.phone,
      sms_code: form.sms_code
    })
    userStore.setToken(res.token)
    userStore.setUserInfo(res)
    ElMessage.success('绑定成功')
    await router.replace('/dashboard')
  } catch (error: any) {
    ElMessage.error(error?.message || '绑定失败')
  } finally {
    submitting.value = false
  }
}
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
  max-width: 460px;
  padding: 36px 32px;
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.card-tip {
  margin: 10px 0 24px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}

.card-note {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

/* 验证码行：按钮随输入框等高，与输入框对齐 */
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
  width: 120px;
  padding: 0;
  font-size: 13px;
  white-space: nowrap;
}
</style>