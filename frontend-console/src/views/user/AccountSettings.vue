<template>
  <div class="account-container">
    <div class="content">
      <div class="card">
        <h3 class="section-title">
          <el-icon><Lock /></el-icon>
          修改密码
        </h3>

        <el-form :model="passwordForm" label-width="100px" class="password-form">
          <el-form-item label="短信验证码">
            <div class="sms-row">
              <el-input v-model="passwordForm.sms_code" placeholder="请输入短信验证码" />
              <el-button
                :disabled="countdown > 0 || !phone"
                class="sms-btn"
                @click="sendCode"
              >
                {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
              </el-button>
            </div>
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="passwordForm.new_password" type="password" placeholder="请输入新密码" show-password />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleChangePassword" :loading="passwordLoading">
              确认修改
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <div class="card">
        <h3 class="section-title">
          <el-icon><ChatDotRound /></el-icon>
          微信绑定
        </h3>
        <p class="section-tip">
          绑定后可使用微信一键登录本账号。微信客户端内可直接授权绑定，其它环境可用微信扫码绑定。
        </p>

        <div class="bind-row">
          <div class="bind-info">
            <span class="bind-name">微信</span>
            <el-tag v-if="binding.mp_bound" type="success" size="small">已绑定</el-tag>
            <el-tag v-else type="info" size="small">未绑定</el-tag>
          </div>
          <el-button v-if="binding.mp_bound" size="small" @click="handleUnbind('mp')">解除绑定</el-button>
          <el-button v-else size="small" type="primary" @click="handleBind">立即绑定</el-button>
        </div>
        <p v-if="!binding.mp_bound" class="bind-note">
          微信客户端内点「立即绑定」直接授权；其它环境将展示二维码，用微信扫码后完成绑定。
        </p>
      </div>
    </div>

    <!-- PC 扫码绑定弹层 -->
    <WechatQrDialog v-model="bindQrVisible" mode="bind" @success="loadBinding" />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userAPI } from '@/api'
import { verifyCaptcha } from '@/utils/captcha'
import WechatQrDialog from '@/components/WechatQrDialog.vue'

const route = useRoute()
const router = useRouter()

const passwordLoading = ref(false)
const countdown = ref(0)
const phone = ref('')

// 微信绑定状态
const binding = ref({ mp_bound: false, nickname: '' })
// 非微信环境用扫码弹层完成绑定
const bindQrVisible = ref(false)
// 微信客户端内可直接完成页面内授权，其它环境需扫码
const inWechat = /MicroMessenger/i.test(navigator.userAgent)

const loadBinding = async () => {
  try {
    const res: any = await userAPI.getWechatBinding()
    binding.value = {
      mp_bound: Boolean(res?.mp_bound),
      nickname: res?.nickname || ''
    }
  } catch (error) {
    console.error(error)
  }
}

const handleBind = async () => {
  if (!inWechat) {
    bindQrVisible.value = true
    return
  }
  try {
    const res: any = await userAPI.wechatBindAuthorize('mp')
    if (!res?.authorize_url) throw new Error('未获取到微信授权地址')
    window.location.href = res.authorize_url
  } catch (error: any) {
    ElMessage.error(error?.message || '发起绑定失败')
  }
}

const handleUnbind = async (scene: string) => {
  try {
    await ElMessageBox.confirm('解除绑定后将无法使用微信一键登录，确认解除？', '解除微信绑定', {
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await userAPI.unbindWechat(scene)
    ElMessage.success('已解除绑定')
    await loadBinding()
  } catch (error: any) {
    ElMessage.error(error?.message || '解除绑定失败')
  }
}

const passwordForm = reactive({
  sms_code: '',
  new_password: ''
})

const sendCode = async () => {
  if (!phone.value) {
    ElMessage.warning('请先登录获取手机号')
    return
  }
  if (countdown.value > 0) return
  try {
    const captcha = await verifyCaptcha()
    await userAPI.sendSMSCode({
      phone: phone.value,
      captcha_payload: captcha.payload,
      scene: 'change_password'
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

const handleChangePassword = async () => {
  if (!passwordForm.sms_code || !passwordForm.new_password) {
    ElMessage.warning('请填写完整信息')
    return
  }

  passwordLoading.value = true
  try {
    const captcha = await verifyCaptcha()
    await userAPI.changePassword({
      sms_code: passwordForm.sms_code,
      new_password: passwordForm.new_password,
      captcha_payload: captcha.payload
    })
    ElMessage.success('密码修改成功')
    passwordForm.sms_code = ''
    passwordForm.new_password = ''
  } catch (error: any) {
    ElMessage.error(error?.message || '修改失败')
  } finally {
    passwordLoading.value = false
  }
}

onMounted(async () => {
  // 绑定回调跳回本页时带 wechat 参数，提示一次并清理地址栏参数
  const wechatFlag = String(route.query.wechat || '')
  if (wechatFlag) {
    if (wechatFlag === 'bound') ElMessage.success('微信绑定成功')
    else if (wechatFlag === 'conflict') ElMessage.error('该微信已绑定其它账号，请先在该账号解除绑定')
    router.replace('/settings')
  }

  await loadBinding()

  try {
    const profile: any = await userAPI.getProfile()
    phone.value = profile.phone || ''
  } catch (error) {
    console.error(error)
  }
})
</script>

<style scoped>
.account-container {
  min-height: 100%;
}

.content {
  max-width: 700px;
  margin: 0 auto;
}

.password-form {
  max-width: 480px;
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

/* ========== 微信绑定 ========== */
.section-tip {
  margin: -4px 0 20px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}

.bind-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border-light);
}

.bind-row:last-of-type {
  border-bottom: none;
}

.bind-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.bind-name {
  font-size: 14px;
  color: var(--text-primary);
}

.bind-note {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-muted);
}
</style>