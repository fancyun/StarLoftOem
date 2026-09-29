<template>
  <div class="kyc-container">
    <div class="content">
      <div class="card kyc-card">
        <el-skeleton v-if="pageLoading" :rows="8" animated />
        <template v-else>
        <!-- 实名类型切换 -->
        <div class="auth-type-switch">
          <el-radio-group v-model="authType" size="large">
            <el-radio-button value="personal">个人实名</el-radio-button>
            <el-radio-button value="enterprise">企业实名</el-radio-button>
          </el-radio-group>
        </div>

        <!-- ================= 个人实名 ================= -->
        <template v-if="authType === 'personal'">
        <!-- 状态 1：已实名（record_status=2） -->
        <div v-if="recordStatus === 2" class="verified-section">
          <el-icon class="success-icon"><SuccessFilled /></el-icon>
          <h2>您已完成个人实名认证</h2>
          <div class="verified-info">
            <p>姓名：{{ maskName(kycName) }}</p>
            <p>身份证号：{{ maskIDCard(kycIDCard) }}</p>
          </div>
          <p class="permanent-tip">实名信息一经认证成功即永久绑定，不可修改。</p>
          <p class="upgrade-tip">如需对接企业级产品，可切换到「企业实名」完成法人认证升级。</p>
          <div class="verified-actions">
            <el-button @click="$router.push('/dashboard')">返回首页</el-button>
          </div>
        </div>

        <!-- 状态 2：进行中（record_status=1） -->
        <div v-if="recordStatus === 1" class="processing-section">
          <el-icon class="processing-icon"><Clock /></el-icon>
          <h2>个人认证进行中</h2>
          <p>如您已完成人脸验证，请点击「查询认证结果」；未完成可继续认证</p>
          <div class="processing-actions">
            <el-button type="primary" size="large" :loading="syncing" @click="syncResult">查询认证结果</el-button>
            <el-button v-if="pendingAuthUrl" size="large" @click="continueAuth">继续认证</el-button>
            <el-button size="large" @click="cancelAuth">取消认证</el-button>
          </div>
        </div>

        <!-- 状态 3：无记录 / 认证失败 / 认证超时 -->
        <div v-if="recordStatus === -1 || recordStatus === 3 || recordStatus === 4 || recordStatus === 5" class="auth-form-section">
          <div class="form-header">
            <h2 v-if="recordStatus === -1">个人实名认证</h2>
            <h2 v-else-if="recordStatus === 3">实名认证失败</h2>
            <h2 v-else-if="recordStatus === 5">认证超时</h2>
            <h2 v-else>个人实名认证</h2>
            <p v-if="recordStatus === -1">完成实名认证后，将为您自动开通 API，可使用 API 进行业务调用</p>
            <p v-else-if="recordStatus === 3">上次实名认证未通过，请重新填写信息后再次认证</p>
            <p v-else-if="recordStatus === 5">上次认证连接超时，请重新填写信息后再试</p>
            <p v-else>请填写姓名和身份证号，重新完成实名认证</p>

            <div class="free-remaining" :class="{ exhausted: freeAuthRemaining <= 0 }">
              <template v-if="freeAuthRemaining > 0">
                个人实名剩余免费认证次数：<strong>{{ freeAuthRemaining }}</strong> 次
              </template>
              <template v-else>
                免费认证次数已用完，后续认证按 {{ kycPersonalPrice }} 元/次计费（优先扣资源包，再扣余额）
              </template>
            </div>
          </div>

          <el-form :model="form" :rules="rules" ref="formRef" label-width="100px" class="auth-form">
            <el-form-item label="真实姓名" prop="name">
              <el-input v-model="form.name" placeholder="请输入真实姓名" />
            </el-form-item>
            <el-form-item label="身份证号" prop="id_card">
              <el-input v-model="form.id_card" placeholder="请输入身份证号" maxlength="18" />
            </el-form-item>
            <el-button type="primary" size="large" class="submit-btn" :loading="loading" @click="handleSubmit">
              开始认证
            </el-button>
            <p class="auth-legal">
              点击「开始认证」即表示您已阅读并同意
              <a :href="`${siteBase('portal')}/auth-authorization`" target="_blank" rel="noopener" class="agree-link">《实名认证授权协议》</a>
            </p>
          </el-form>

          <div class="tips">
            <h3>认证说明</h3>
            <ul>
              <li>请确保提供的姓名和身份证号真实有效</li>
              <li>认证过程中需要进行人脸识别，请在光线充足的环境下操作</li>
              <li>个人实名有 3 次免费认证次数（失败也占用），用尽后按 {{ kycPersonalPrice }} 元/次计费（优先扣资源包，再扣余额）</li>
              <li>实名信息一经认证成功即永久绑定，不可修改</li>
            </ul>
          </div>
        </div>
        </template>

        <!-- ================= 企业实名 ================= -->
        <template v-else>
        <!-- 状态 1：已通过（record_status=2） -->
        <div v-if="entRecordStatus === 2" class="verified-section">
          <el-icon class="success-icon"><SuccessFilled /></el-icon>
          <h2>您已完成企业实名认证</h2>
          <div class="verified-info">
            <p>企业名称：{{ entCompany }}</p>
            <p>统一社会信用代码：{{ maskCreditCode(entCreditCode) }}</p>
          </div>
          <p class="permanent-tip">实名信息一经认证成功即永久绑定，不可修改。</p>
          <div class="verified-actions">
            <el-button @click="$router.push('/dashboard')">返回首页</el-button>
          </div>
        </div>

        <!-- 状态 2：进行中（record_status=0/1，待四要素或待法人扫脸） -->
        <div v-if="entRecordStatus === 0 || entRecordStatus === 1" class="processing-section">
          <el-icon class="processing-icon"><Clock /></el-icon>
          <h2>企业认证进行中</h2>
          <p v-if="entRecordStatus === 0">企业信息核验进行中，如已完成核验请点击「查询认证结果」</p>
          <p v-else>如您已完成法人扫脸，请点击「查询认证结果」；未完成可继续认证</p>
          <div class="processing-actions">
            <el-button type="primary" size="large" :loading="entSyncing" @click="refreshEnterprise">查询认证结果</el-button>
            <el-button v-if="entPendingUrl" size="large" @click="continueEnterprise">继续认证</el-button>
          </div>
        </div>

        <!-- 状态 3：无记录 / 未通过（record_status=-1 / 3） -->
        <div v-if="entRecordStatus === -1 || entRecordStatus === 3" class="auth-form-section">
          <div class="form-header">
            <h2 v-if="entRecordStatus === -1">企业实名认证</h2>
            <h2 v-else>企业实名认证失败</h2>
            <p v-if="entRecordStatus === -1">填写企业与法人信息，先进行企业信息核验，核验通过后由法人完成人脸核验；通过后将为您开通企业级 API 权限</p>
            <p v-else>上次企业认证未通过，请核对信息后重新提交</p>

            <div class="free-remaining" :class="{ exhausted: entFreeAuthRemaining <= 0 }">
              <template v-if="entFreeAuthRemaining > 0">
                企业实名剩余免费认证次数：<strong>{{ entFreeAuthRemaining }}</strong> 次
              </template>
              <template v-else>
                免费认证次数已用完，后续认证按 {{ kycEnterprisePrice }} 元/次计费（优先扣资源包，再扣余额）
              </template>
            </div>
          </div>

          <el-form :model="entForm" :rules="entRules" ref="entFormRef" label-width="130px" class="auth-form">
            <el-form-item label="企业名称" prop="company_name">
              <el-input v-model="entForm.company_name" placeholder="请输入与营业执照一致的企业名称" />
            </el-form-item>
            <el-form-item label="统一社会信用代码" prop="credit_code">
              <el-input v-model="entForm.credit_code" placeholder="请输入18位统一社会信用代码" maxlength="18" />
            </el-form-item>
            <el-form-item label="法人姓名" prop="legal_name">
              <el-input v-model="entForm.legal_name" placeholder="请输入法定代表人姓名" />
            </el-form-item>
            <el-form-item label="法人身份证号" prop="legal_id_card">
              <el-input v-model="entForm.legal_id_card" placeholder="请输入法定代表人身份证号" maxlength="18" />
            </el-form-item>
            <el-button type="primary" size="large" class="submit-btn" :loading="entLoading" @click="handleEnterpriseSubmit">
              开始认证
            </el-button>
          </el-form>

          <div class="tips">
            <h3>认证说明</h3>
            <ul>
              <li>企业实名需先通过企业信息（四要素）核验，再由法定代表人本人完成人脸核验，请准备好法人身份证</li>
              <li>认证过程中需要进行人脸识别，请在光线充足的环境下操作</li>
              <li>企业信息核验不通过也会占用免费认证次数</li>
              <li>企业实名有 3 次免费认证次数，用尽后按 {{ kycEnterprisePrice }} 元/次计费（优先扣资源包，再扣余额）</li>
              <li>工商四要素与法人扫脸均通过后方视为企业实名成功</li>
              <li>实名信息一经认证成功即永久绑定，不可修改</li>
            </ul>
          </div>
        </div>
        </template>
        </template>
      </div>
    </div>

    <!-- 扫码认证弹窗（PC 端发起 faceid 时展示二维码，手机扫码完成核身） -->
    <el-dialog v-model="qrVisible" title="扫码完成认证" width="340px" :close-on-click-modal="false" align-center>
      <div class="qr-box">
        <qrcode-vue v-if="qrUrl" :value="qrUrl" :size="190" level="H" />
        <p class="qr-tip">请使用手机扫码，完成人脸核身认证</p>
        <div class="qr-actions">
          <el-button type="primary" size="large" :loading="qrSyncing" @click="handleQrDone">我已扫码完成</el-button>
          <el-button size="large" @click="closeQr">稍后处理</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import QrcodeVue from 'qrcode.vue'
import { userAPI } from '@/api'
import { siteBase } from '@/utils/promotion'

const loading = ref(false)
const syncing = ref(false)
const pageLoading = ref(true)
const recordStatus = ref(-1)  // -1=无记录, 1=进行中, 2=已实名, 3=失败
const kycName = ref('')
const kycIDCard = ref('')
const pendingAuthUrl = ref('')
// 扫码认证弹窗（PC 端发起 faceid 时展示二维码，手机扫码完成核身）
const qrVisible = ref(false)
const qrUrl = ref('')
const qrSyncing = ref(false)
// 个人实名剩余免费认证次数
const freeAuthRemaining = ref(0)
// 个人/企业实名超次单价（元/次）
const kycPersonalPrice = ref(1)
const kycEnterprisePrice = ref(2)
// 实名类型切换
const authType = ref<'personal' | 'enterprise'>('personal')

// 认证完成后的回跳地址：账户级实名页（实名绑定账户，不区分产品）
const returnUrl = () => {
  return window.location.origin + '/certification'
}

const form = reactive({
  name: '',
  id_card: '',
  return_url: returnUrl()
})

const validateIdCard = (_rule: any, value: string, callback: any) => {
  const reg = /(^\d{15}$)|(^\d{18}$)|(^\d{17}(\d|X|x)$)/
  if (!reg.test(value)) {
    callback(new Error('请输入正确的身份证号'))
  } else {
    callback()
  }
}

const validateCreditCode = (_rule: any, value: string, callback: any) => {
  const reg = /^[0-9A-HJ-NPQRTUWXY]{18}$/
  if (!reg.test(value)) {
    callback(new Error('请输入正确的18位统一社会信用代码'))
  } else {
    callback()
  }
}

const rules = {
  name: [
    { required: true, message: '请输入真实姓名', trigger: 'blur' },
    { min: 2, max: 20, message: '姓名长度在 2 到 20 个字符', trigger: 'blur' }
  ],
  id_card: [
    { required: true, message: '请输入身份证号', trigger: 'blur' },
    { validator: validateIdCard, trigger: 'blur' }
  ]
}

const formRef = ref()

const maskName = (name: string) => {
  if (!name) return ''
  return name.charAt(0) + '**'
}

const maskIDCard = (idCard: string) => {
  if (!idCard) return ''
  return idCard.substring(0, 3) + '***********' + idCard.substring(idCard.length - 4)
}

/* ---------------- 扫码/跳转分流 ---------------- */
// 移动端直接跳转核身页，PC 端展示二维码供手机扫码完成核身
const isMobile = () => {
  return (
    /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
    (navigator.maxTouchPoints > 1 && window.innerWidth < 768)
  )
}

const handleAuthRedirect = (authUrl: string) => {
  if (isMobile()) {
    window.location.href = authUrl
  } else {
    qrUrl.value = authUrl
    qrVisible.value = true
  }
}

// 稍后处理：关闭弹窗并刷新认证状态（发起后进入「认证进行中」）
const closeQr = () => {
  qrVisible.value = false
  loadData()
  loadEnterpriseData()
}

// 我已扫码完成：关闭弹窗并按当前实名类型同步核身结果
const handleQrDone = async () => {
  qrVisible.value = false
  qrSyncing.value = true
  try {
    if (authType.value === 'enterprise') {
      await loadEnterpriseData()
    } else {
      await syncResult()
    }
  } finally {
    qrSyncing.value = false
  }
}

/* ---------------- 个人实名 ---------------- */
const handleSubmit = async () => {
  await formRef.value.validate()

  loading.value = true
  try {
    const res = await userAPI.startKyc({
      name: form.name,
      id_card: form.id_card,
      return_url: returnUrl()
    })

    if (res && res.auth_url) {
      handleAuthRedirect(res.auth_url)
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '实名认证发起失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

const continueAuth = () => {
  if (pendingAuthUrl.value) {
    window.location.href = pendingAuthUrl.value
  } else {
    ElMessage.warning('暂无认证地址，请稍后重试')
  }
}

const syncResult = async () => {
  syncing.value = true
  try {
    await userAPI.syncKyc()
    await loadData()
  } catch (error) {
    console.error(error)
    ElMessage.error('查询失败，请稍后重试')
  } finally {
    syncing.value = false
  }
}

const cancelAuth = async () => {
  try {
    await ElMessageBox.confirm('确定要取消当前认证吗？取消后可重新填写信息进行认证。', '取消认证', {
      confirmButtonText: '确定取消',
      cancelButtonText: '我再想想',
      type: 'warning'
    })
    await userAPI.cancelKyc()
    ElMessage.success('认证已取消')
    recordStatus.value = 3
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('取消失败，请稍后重试')
    }
  }
}

const loadData = async () => {
  try {
    const data = (await userAPI.getKycStatus()) as any
    recordStatus.value = data.record_status ?? -1
    pendingAuthUrl.value = data.pending_auth_url || ''

    if (data.verified_name) kycName.value = data.verified_name
    if (data.verified_number) kycIDCard.value = data.verified_number
  } catch (error) {
    console.error(error)
  }
}

const loadFreeAuthRemaining = async () => {
  try {
    const profile: any = await userAPI.getProfile()
    freeAuthRemaining.value = profile.free_auth_remaining ?? 0
    kycPersonalPrice.value = profile.kyc_personal_price ?? 1
    kycEnterprisePrice.value = profile.kyc_enterprise_price ?? 2
  } catch (error) {
    console.error(error)
  }
}

/* ---------------- 企业实名 ---------------- */
const entLoading = ref(false)
const entSyncing = ref(false)
const entRecordStatus = ref(-1) // -1=无记录 0=待四要素 1=待法人扫脸 2=通过 3=未通过
const entCompany = ref('')
const entCreditCode = ref('')
const entPendingUrl = ref('')
// 企业实名剩余免费认证次数
const entFreeAuthRemaining = ref(0)

const entForm = reactive({
  company_name: '',
  credit_code: '',
  legal_name: '',
  legal_id_card: '',
  return_url: returnUrl()
})

const entRules = {
  company_name: [{ required: true, message: '请输入企业名称', trigger: 'blur' }],
  credit_code: [
    { required: true, message: '请输入统一社会信用代码', trigger: 'blur' },
    { validator: validateCreditCode, trigger: 'blur' }
  ],
  legal_name: [{ required: true, message: '请输入法人姓名', trigger: 'blur' }],
  legal_id_card: [
    { required: true, message: '请输入法人身份证号', trigger: 'blur' },
    { validator: validateIdCard, trigger: 'blur' }
  ]
}

const entFormRef = ref()

const maskCreditCode = (code: string) => {
  if (!code) return ''
  return code.substring(0, 4) + '**********' + code.substring(code.length - 4)
}

const handleEnterpriseSubmit = async () => {
  await entFormRef.value.validate()

  entLoading.value = true
  try {
    const res = await userAPI.startKyb({
      company_name: entForm.company_name,
      credit_code: entForm.credit_code,
      legal_name: entForm.legal_name,
      legal_id_card: entForm.legal_id_card,
      return_url: returnUrl()
    })

    if (res && res.auth_url) {
      handleAuthRedirect(res.auth_url)
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '企业认证发起失败，请稍后重试')
  } finally {
    entLoading.value = false
  }
}

const loadEnterpriseData = async () => {
  entSyncing.value = true
  try {
    const data = (await userAPI.getKybStatus()) as any
    entRecordStatus.value = data.record_status ?? -1
    entPendingUrl.value = data.pending_auth_url || ''
    entFreeAuthRemaining.value = data.free_auth_remaining ?? 0
    if (data.company_name) entCompany.value = data.company_name
    if (data.credit_code) entCreditCode.value = data.credit_code
  } catch (error) {
    console.error(error)
  } finally {
    entSyncing.value = false
  }
}

const refreshEnterprise = () => {
  loadEnterpriseData()
}

const continueEnterprise = () => {
  if (entPendingUrl.value) {
    window.location.href = entPendingUrl.value
  } else {
    ElMessage.warning('暂无认证地址，请稍后重试')
  }
}

onMounted(async () => {
  pageLoading.value = true
  await Promise.all([loadData(), loadEnterpriseData(), loadFreeAuthRemaining()])
  // 已企业实名时默认展示企业实名结果
  if (entRecordStatus.value === 2) {
    authType.value = 'enterprise'
  }
  pageLoading.value = false
})
</script>

<style scoped>
.kyc-container {
  min-height: 100%;
}
.content {
  max-width: 800px;
  margin: 0 auto;
}
.kyc-card {
  padding: 40px;
}
.auth-type-switch {
  display: flex;
  justify-content: center;
  margin-bottom: 32px;
}

/* 已实名 */
.verified-section {
  text-align: center;
}
.success-icon {
  width: 72px; height: 72px;
  border-radius: 50%;
  background: var(--color-success-light);
  color: var(--color-success);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 40px;
  margin-bottom: 16px;
}
.verified-section h2 {
  font-size: 24px;
  color: var(--text-primary);
  margin-bottom: 24px;
}
.verified-info {
  margin-bottom: 32px;
  padding: 16px 24px;
  background: var(--bg-soft);
  border-radius: var(--radius-md);
  display: inline-block;
}
.verified-info p {
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 8px;
}
.verified-info p:last-child { margin-bottom: 0; }
.verified-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}
.upgrade-tip {
  color: var(--color-primary);
  font-size: 13px;
  margin-bottom: 16px;
}

/* 进行中 */
.processing-section {
  text-align: center;
  padding: 40px 0;
}
.processing-icon {
  width: 72px; height: 72px;
  border-radius: 50%;
  background: var(--color-warning-light);
  color: var(--color-warning);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 40px;
  margin-bottom: 16px;
}
.processing-section h2 {
  font-size: 24px;
  color: var(--text-primary);
  margin-bottom: 12px;
}
.processing-section p {
  color: var(--text-muted);
  font-size: 14px;
  margin-bottom: 24px;
}
.processing-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}

/* 未认证/失败 */
.form-header {
  text-align: center;
  margin-bottom: 32px;
}
.form-header h2 {
  font-size: 24px;
  color: var(--text-primary);
  margin-bottom: 8px;
}
.form-header p {
  color: var(--text-muted);
  font-size: 14px;
}
.free-remaining {
  margin-top: 16px;
  display: inline-block;
  padding: 8px 20px;
  border-radius: var(--radius-md);
  background: var(--color-success-light);
  color: var(--color-success);
  font-size: 14px;
}
.free-remaining strong {
  font-size: 18px;
}
.free-remaining.exhausted {
  background: var(--color-warning-light);
  color: var(--color-warning);
}
.auth-form {
  margin-bottom: 32px;
}
.submit-btn {
  width: 100%;
  height: 48px;
  font-size: 16px;
}
.auth-legal {
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-muted);
  text-align: center;
}
.auth-legal .agree-link {
  color: var(--color-primary);
  text-decoration: none;
}
.tips {
  padding: 24px;
  background: var(--bg-soft);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}
.tips h3 {
  font-size: 16px;
  color: var(--text-primary);
  margin-bottom: 16px;
}
.tips ul {
  list-style: none;
  padding: 0;
}
.tips li {
  color: var(--text-secondary);
  font-size: 13px;
  margin-bottom: 8px;
  padding-left: 16px;
  position: relative;
}
.tips li::before {
  content: '•';
  position: absolute;
  left: 0;
  color: var(--color-primary);
  font-size: 16px;
  line-height: 1.4;
}

/* 扫码认证弹窗 */
.qr-box {
  text-align: center;
}
.qr-box :deep(svg) {
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  padding: 8px;
}
.qr-tip {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 16px 0 20px;
}
.qr-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}

</style>