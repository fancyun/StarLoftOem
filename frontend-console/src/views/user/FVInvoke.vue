<template>
  <div class="invoke-container page-fill">
    <div class="content">
      <div class="card">
        <h3 class="section-title">
          <el-icon><Aim /></el-icon>
          在线调用
        </h3>
        <p class="invoke-tip">
          在线发起一次真实人脸核验：提交后生成核验链接与二维码（15 分钟内有效），用手机扫码完成活体核身，
          结果实时回显在本页，也可在「认证记录」中查询。本次调用{{ priceText }}，优先扣减人脸核验资源包次数，不足则扣余额。
        </p>

        <el-form :model="form" label-width="90px" class="invoke-form">
          <el-form-item label="核验类型">
            <el-radio-group v-model="form.product">
              <el-radio value="fv_auth">有源核验（人脸 + 公安库）</el-radio>
              <el-radio value="fv_self">无源核验（人脸 + 留底照片）</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item v-if="form.product === 'fv_auth'" label="姓名">
            <el-input v-model="form.name" placeholder="被核验人真实姓名" style="max-width: 320px" />
          </el-form-item>
          <el-form-item v-if="form.product === 'fv_auth'" label="身份证号">
            <el-input v-model="form.id_card" placeholder="被核验人身份证号" style="max-width: 320px" />
          </el-form-item>
          <el-button type="primary" :loading="submitting" @click="handleStart">发起核验</el-button>
        </el-form>
      </div>

      <div v-if="result" class="card">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          核验进度
        </h3>
        <div class="invoke-result">
          <div class="qr-box">
            <qrcode-vue :value="result.site_url" :size="180" level="H" />
            <p class="qr-tip">请用手机扫码完成核身</p>
          </div>
          <div class="result-info">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="流水号">{{ result.biz_no }}</el-descriptions-item>
              <el-descriptions-item label="核验类型">{{ productLabel }}</el-descriptions-item>
              <el-descriptions-item label="状态">
                <el-tag :type="statusType">{{ statusText }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="结果">{{ currentRecord?.result_message || '-' }}</el-descriptions-item>
              <el-descriptions-item label="链接有效期至">{{ expireText }}</el-descriptions-item>
            </el-descriptions>
            <div class="result-actions">
              <el-button @click="copyLink">复制核验链接</el-button>
              <el-button @click="pollStatus">刷新状态</el-button>
            </div>
            <p class="result-hint">链接每次核验仅可使用一次；未在有效期内完成的核验不会扣费。</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import QrcodeVue from 'qrcode.vue'
import { publicAPI, userAPI, promotionAPI } from '@/api'

const form = ref({ product: 'fv_auth', name: '', id_card: '' })
const submitting = ref(false)
const result = ref<any>(null)
const currentRecord = ref<any>(null)
const prices = ref({ auth: 0, self: 0 })

let pollTimer: ReturnType<typeof setInterval> | null = null

// 终态：认证成功/失败/已取消/超时结束/发起失败
const TERMINAL_STATUS = [2, 3, 4, 5, 6]

const priceText = computed(() => {
  const price = form.value.product === 'fv_self' ? prices.value.self : prices.value.auth
  return price > 0 ? `按 ¥${price}/次计费` : '按平台单价计费'
})

const productLabel = computed(() => (result.value?.product === 'fv_self' ? '无源核验' : '有源核验'))

const statusText = computed(() => {
  const status = currentRecord.value?.status ?? 0
  if (status === 5) return currentRecord.value?.is_refunded ? '超时已退款' : '超时未计费'
  const map: Record<number, string> = { 0: '待认证', 1: '认证中', 2: '认证成功', 3: '认证失败', 4: '已取消', 6: '发起失败（未扣费）' }
  return map[status] || '待认证'
})

const statusType = computed(() => {
  const map: Record<number, string> = { 1: 'warning', 2: 'success', 3: 'danger', 4: 'info', 5: 'info', 6: 'info' }
  return map[currentRecord.value?.status ?? 0] || 'info'
})

const pad = (n: number) => String(n).padStart(2, '0')

const formatLocalTime = (date: Date) =>
  `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`

const expireText = computed(() => {
  const seconds = Number(result.value?.expired_in || 0)
  if (seconds > 0) return formatLocalTime(new Date(Date.now() + seconds * 1000))
  const ts = Number(result.value?.expired_time || 0)
  return ts > 0 ? formatLocalTime(new Date(ts * 1000)) : '-'
})

const stopPoll = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 按流水号在最近认证记录中查询本次核验状态
const pollStatus = async () => {
  if (!result.value?.biz_no) return
  try {
    const res: any = await userAPI.getFvRecords({ page: 1, page_size: 20 })
    const hit = (res?.list || []).find((row: any) => row.biz_no === result.value.biz_no)
    if (!hit) return
    currentRecord.value = hit
    if (TERMINAL_STATUS.includes(hit.status)) stopPoll()
  } catch (error) {
    console.error(error)
  }
}

const startPoll = () => {
  pollStatus()
  pollTimer = setInterval(pollStatus, 3000)
}

const handleStart = async () => {
  if (form.value.product === 'fv_auth') {
    if (!form.value.name.trim()) {
      ElMessage.warning('请输入被核验人姓名')
      return
    }
    if (!form.value.id_card.trim()) {
      ElMessage.warning('请输入被核验人身份证号')
      return
    }
  }
  submitting.value = true
  stopPoll()
  try {
    const res: any = await userAPI.startFv({
      product: form.value.product,
      name: form.value.product === 'fv_auth' ? form.value.name.trim() : '',
      id_card: form.value.product === 'fv_auth' ? form.value.id_card.trim() : ''
    })
    if (!res?.site_url) {
      ElMessage.error('发起核验失败，请稍后重试')
      return
    }
    result.value = { ...res, product: form.value.product }
    currentRecord.value = null
    startPoll()
  } catch (error: any) {
    ElMessage.error(error?.message || '发起核验失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

const copyLink = async () => {
  if (!result.value?.site_url) return
  try {
    await navigator.clipboard.writeText(result.value.site_url)
    ElMessage.success('链接已复制，请在手机浏览器中打开')
  } catch (error) {
    console.error(error)
    ElMessage.warning('复制失败，请手动选择链接复制')
  }
}

// 单价取用户适用价（推广/用户级覆盖优先，回落平台价）
const loadPrices = async () => {
  try {
    const profile: any = await promotionAPI.profile()
    const unit = profile?.unit_prices || {}
    prices.value = {
      auth: Number(unit.fv_auth || 0),
      self: Number(unit.fv_self || 0),
    }
  } catch (error) {
    console.error(error)
    try {
      const config: any = await publicAPI.getConfig()
      prices.value = { auth: Number(config?.fv_auth_price || 0), self: Number(config?.fv_self_price || 0) }
    } catch (innerError) {
      console.error(innerError)
    }
  }
}

onMounted(loadPrices)
onBeforeUnmount(stopPoll)
</script>

<style scoped>
.content {
  max-width: 900px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.invoke-tip {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.8;
  margin: 0 0 20px;
  padding: 10px 14px;
  background: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.invoke-result {
  display: flex;
  gap: 32px;
  align-items: flex-start;
}

.qr-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px;
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  background: var(--bg-page);
}

.qr-tip {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
}

.result-info {
  flex: 1;
  min-width: 0;
}

.result-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}

.result-hint {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 8px 0 0;
}

@media (max-width: 768px) {
  .invoke-result {
    flex-direction: column;
    align-items: stretch;
  }

  .qr-box {
    align-self: center;
  }
}
</style>