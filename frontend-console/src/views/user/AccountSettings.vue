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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { userAPI } from '@/api'
import { verifyCaptcha } from '@/utils/captcha'

const passwordLoading = ref(false)
const countdown = ref(0)
const phone = ref('')

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
    const { ticket, randstr } = await verifyCaptcha()
    await userAPI.sendSMSCode({
      phone: phone.value,
      captcha_ticket: ticket,
      captcha_randstr: randstr,
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
    const { ticket, randstr } = await verifyCaptcha()
    await userAPI.changePassword({
      sms_code: passwordForm.sms_code,
      new_password: passwordForm.new_password,
      captcha_ticket: ticket,
      captcha_randstr: randstr
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
</style>