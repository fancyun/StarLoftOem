<template>
  <el-dialog
    :model-value="modelValue"
    :title="mode === 'login' ? '微信扫码登录' : '微信扫码绑定'"
    width="420px"
    align-center
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
    @closed="handleClosed"
  >
    <!-- 等待扫码：展示二维码，用户用微信扫一扫在微信内置浏览器中完成授权 -->
    <div v-if="status !== 'need_bind'" class="qr-wrap">
      <div class="qr-box">
        <qrcode-vue v-if="qrUrl && !expired" :value="qrUrl" :size="196" level="M" />
        <div v-else-if="expired" class="qr-expired">
          <p>二维码已过期</p>
          <el-button size="small" type="primary" :loading="creating" @click="createSession">刷新二维码</el-button>
        </div>
        <el-icon v-else class="qr-loading"><Loading /></el-icon>
      </div>
      <p class="qr-tip">
        <template v-if="mode === 'login'">
          请用微信「扫一扫」扫描二维码，在手机上确认授权后，本页面将自动登录。
        </template>
        <template v-else>请用微信「扫一扫」扫描二维码，扫码授权后即完成绑定。</template>
      </p>
      <p v-if="status === 'pending'" class="qr-status">等待扫码…</p>
    </div>

    <!-- 该微信未绑定账号：在电脑上完成手机号 + 短信验证码绑定 -->
    <div v-else class="bind-wrap">
      <p class="bind-tip">该微信尚未绑定平台账号，请用平台注册手机号验证身份完成绑定。</p>
      <el-form label-width="90px">
        <el-form-item label="手机号">
          <el-input v-model="bindForm.phone" placeholder="请输入平台注册手机号" clearable />
        </el-form-item>
        <el-form-item label="短信验证码">
          <div class="sms-row">
            <el-input v-model="bindForm.sms_code" placeholder="请输入短信验证码" />
            <el-button :disabled="countdown > 0 || !bindForm.phone" class="sms-btn" @click="sendCode">
              {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <el-button type="primary" :loading="submitting" @click="handleBind">绑定并登录</el-button>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import QrcodeVue from 'qrcode.vue'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { verifyCaptcha } from '@/utils/captcha'

const props = defineProps<{ modelValue: boolean; mode: 'login' | 'bind' }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
  (e: 'success'): void
}>()

const userStore = useUserStore()

const qrUrl = ref('')
const qrTicket = ref('')
const bindTicket = ref('')
const status = ref<'pending' | 'need_bind' | 'authorized' | 'expired'>('pending')
const creating = ref(false)
const submitting = ref(false)
const countdown = ref(0)
const bindForm = reactive({ phone: '', sms_code: '' })

let pollTimer: number | null = null
let pollIntervalMs = 2000
let countdownTimer: number | null = null

const expired = computed(() => status.value === 'expired')

// 创建扫码会话（登录 / 绑定），二维码内容为公众号网页授权链接
const createSession = async () => {
  if (creating.value) return
  creating.value = true
  try {
    const res: any =
      props.mode === 'login' ? await userAPI.createWechatQRSession() : await userAPI.createWechatQRBindSession()
    qrUrl.value = String(res?.qr_url || '')
    qrTicket.value = String(res?.qr_ticket || '')
    bindTicket.value = ''
    status.value = 'pending'
    if (Number(res?.poll_interval) > 0) pollIntervalMs = Number(res.poll_interval) * 1000
    startPolling()
  } catch (error: any) {
    ElMessage.error(error?.message || '发起微信扫码失败')
  } finally {
    creating.value = false
  }
}

const stopPolling = () => {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

const startPolling = () => {
  stopPolling()
  pollTimer = window.setInterval(poll, pollIntervalMs)
}

const poll = async () => {
  if (!qrTicket.value) return
  try {
    const res: any = await userAPI.wechatQRPoll({ qr_ticket: qrTicket.value })
    const st = String(res?.status || '')
    if (st === 'pending' || !st) {
      status.value = 'pending'
    } else if (st === 'need_bind') {
      // 未绑定：停止轮询，改由本机提交手机号 + 短信验证码完成绑定
      status.value = 'need_bind'
      bindTicket.value = String(res?.bind_ticket || '')
      stopPolling()
    } else if (st === 'authorized') {
      stopPolling()
      await finishLogin(String(res?.login_ticket || ''))
    } else if (st === 'bound') {
      stopPolling()
      ElMessage.success('微信绑定成功')
      emit('success')
      close()
    } else if (st === 'conflict') {
      stopPolling()
      ElMessage.error('该微信已绑定其它账号，请先在该账号解绑')
    } else if (st === 'expired') {
      status.value = 'expired'
      stopPolling()
    }
  } catch {
    // 轮询失败（限流/网络抖动）跳过本轮，等待下一次
  }
}

// 已授权：用一次性登录票据换取登录态（PC 是唯一拿到登录态的一端）
const finishLogin = async (loginTicket: string) => {
  if (!loginTicket) {
    ElMessage.error('登录票据缺失，请重新扫码')
    return
  }
  try {
    const res: any = await userAPI.wechatTicket({ ticket: loginTicket })
    userStore.setToken(res.token)
    userStore.setUserInfo(res)
    ElMessage.success('登录成功')
    emit('success')
    close()
  } catch (error: any) {
    ElMessage.error(error?.message || '登录失败')
  }
}

const sendCode = async () => {
  if (!/^1[3-9]\d{9}$/.test(bindForm.phone)) {
    ElMessage.warning('请输入正确的手机号')
    return
  }
  if (countdown.value > 0) return
  try {
    const captcha = await verifyCaptcha()
    await userAPI.sendSMSCode({ phone: bindForm.phone, captcha_payload: captcha.payload, scene: 'login' })
    ElMessage.success('验证码已发送')
    countdown.value = 60
    countdownTimer = window.setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0 && countdownTimer !== null) {
        window.clearInterval(countdownTimer)
        countdownTimer = null
      }
    }, 1000)
  } catch (error: any) {
    if (error?.message !== '用户取消验证') ElMessage.error(error?.message || '验证码发送失败')
  }
}

const handleBind = async () => {
  if (!bindTicket.value) {
    ElMessage.error('绑定信息已失效，请重新扫码')
    return
  }
  if (!/^1[3-9]\d{9}$/.test(bindForm.phone) || !bindForm.sms_code) {
    ElMessage.warning('请填写手机号与短信验证码')
    return
  }
  submitting.value = true
  try {
    const res: any = await userAPI.wechatBind({
      bind_ticket: bindTicket.value,
      phone: bindForm.phone,
      sms_code: bindForm.sms_code
    })
    userStore.setToken(res.token)
    userStore.setUserInfo(res)
    ElMessage.success('绑定成功')
    emit('success')
    close()
  } catch (error: any) {
    ElMessage.error(error?.message || '绑定失败')
  } finally {
    submitting.value = false
  }
}

const close = () => emit('update:modelValue', false)

const reset = () => {
  stopPolling()
  qrUrl.value = ''
  qrTicket.value = ''
  bindTicket.value = ''
  bindForm.phone = ''
  bindForm.sms_code = ''
  status.value = 'pending'
}

const handleClosed = () => reset()

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) createSession()
    else reset()
  }
)

onBeforeUnmount(() => {
  stopPolling()
  if (countdownTimer !== null) window.clearInterval(countdownTimer)
})
</script>

<style scoped>
.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.qr-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 220px;
  height: 220px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.qr-expired {
  text-align: center;
  font-size: 13px;
  color: var(--text-muted);
}

.qr-expired p {
  margin-bottom: 10px;
}

.qr-loading {
  font-size: 24px;
  color: var(--text-muted);
  animation: qr-spin 1.2s linear infinite;
}

@keyframes qr-spin {
  to {
    transform: rotate(360deg);
  }
}

.qr-tip {
  margin: 14px 0 0;
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
  color: var(--text-muted);
}

.qr-status {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.bind-tip {
  margin-bottom: 18px;
  font-size: 13px;
  line-height: 1.6;
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