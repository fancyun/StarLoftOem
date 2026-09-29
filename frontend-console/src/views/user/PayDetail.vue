<template>
  <div class="pay-detail-container">
    <div class="content" v-loading="pageLoading">
      <div class="card">
        <h3 class="section-title">
          <el-icon><Wallet /></el-icon>
          支付详情
        </h3>

        <!-- 已支付 -->
        <div v-if="paid" class="result-box">
          <el-icon class="result-icon success"><CircleCheck /></el-icon>
          <div class="result-title">支付成功</div>
          <div class="result-amount">+¥{{ order.amount }}</div>
          <el-button type="primary" @click="goBack">{{ resultBackText }}</el-button>
        </div>

        <!-- 已关闭/已退款 -->
        <div v-else-if="closed" class="result-box">
          <el-icon class="result-icon"><CircleClose /></el-icon>
          <div class="result-title">{{ statusText }}</div>
          <el-button type="primary" @click="goBack">{{ resultBackText }}</el-button>
        </div>

        <!-- 待支付 -->
        <template v-else>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="支付单号">{{ payOrderNo }}</el-descriptions-item>
            <el-descriptions-item label="支付金额">¥{{ order.amount ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="支付方式">{{ channelText }}</el-descriptions-item>
            <el-descriptions-item label="订单状态">待支付</el-descriptions-item>
          </el-descriptions>

          <div v-if="payData" class="pay-action">
            <!-- 微信 Native：展示二维码 -->
            <div v-if="payData.pay_type === 'native'" class="qr-box">
              <qrcode-vue v-if="payData.code_url" :value="payData.code_url" :size="190" level="H" />
              <p class="qr-tip">请使用微信扫码完成支付</p>
            </div>
            <!-- 支付宝：前往支付 -->
            <el-button
              v-else-if="payData.pay_type === 'url'"
              type="primary"
              size="large"
              class="pay-btn"
              @click="openPayUrl"
            >
              前往支付宝支付
            </el-button>
            <!-- 微信 H5：跳转支付 -->
            <el-button
              v-else-if="payData.pay_type === 'h5'"
              type="primary"
              size="large"
              class="pay-btn"
              @click="openH5"
            >
              前往支付
            </el-button>
          </div>
          <p v-else class="muted tip">支付信息已失效，请重新发起</p>

          <div class="pay-actions">
            <el-button type="primary" :loading="checking" @click="checkPaid">查询支付结果</el-button>
            <el-button @click="goBack">返回</el-button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import QrcodeVue from 'qrcode.vue'
import { userAPI } from '@/api'
import { PAY_CHANNEL_LABELS } from '@/utils/payment'

const route = useRoute()
const router = useRouter()

const payOrderNo = computed(() => String(route.params.pay_order_no || ''))

const pageLoading = ref(true)
const order = ref<any>({})
const paid = ref(false)
const closed = ref(false)
const checking = ref(false)

// 一次性的支付信息（创建充值时的返回，存于 sessionStorage，刷新不丢失）
const payData = ref<any>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollCount = 0
const POLL_MAX = 120 // 最多轮询约5分钟

const channelText = computed(() => {
  return PAY_CHANNEL_LABELS[order.value.channel] || order.value.channel || '-'
})

// 按支付用途决定成功后返回目标（充值→余额管理；资源包→我的资源包）
const resultBackText = computed(() => {
  const purpose = payData.value?.pay_purpose
  if (purpose === 'resource_pack') return '返回资源包'
  return '返回余额管理'
})

const statusText = computed(() => {
  if (order.value.status === 3) return '订单已关闭'
  if (order.value.status === 2) return '订单已退款'
  return '订单已结束'
})

const goBack = () => {
  const path = payData.value?.return_path
  clearPayData()
  router.push(path || '/balance')
}

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  pollCount = 0
}

const startPolling = () => {
  stopPolling()
  pollTimer = setInterval(async () => {
    pollCount++
    if (pollCount > POLL_MAX) {
      stopPolling()
      return
    }
    await queryResult()
  }, 2500)
}

const queryResult = async () => {
  try {
    const res = await userAPI.getRechargeResult({ pay_order_no: payOrderNo.value })
    order.value = res
    if (res.status === 1) {
      stopPolling()
      paid.value = true
      clearPayData()
    } else if (res.status === 3 || res.status === 2) {
      stopPolling()
      closed.value = true
    }
  } catch (error) {
    // 查询失败忽略，继续尝试
  }
}

// 用户点击「查询支付结果」时立即查询一次
const checkPaid = async () => {
  checking.value = true
  try {
    await queryResult()
    if (!paid.value && !closed.value) {
      ElMessage.info('尚未检测到支付，请确认已完成支付后重试')
    }
  } finally {
    checking.value = false
  }
}

const openPayUrl = () => {
  if (payData.value?.pay_url) window.open(payData.value.pay_url, '_blank')
}

const openH5 = () => {
  if (payData.value?.h5_url) window.location.href = payData.value.h5_url
}

const clearPayData = () => {
  if (payOrderNo.value) {
    sessionStorage.removeItem(`starloft_pay_${payOrderNo.value}`)
  }
}

onMounted(async () => {
  // 读取创建充值时的支付信息（含一次性支付链接/二维码）
  try {
    const raw = sessionStorage.getItem(`starloft_pay_${payOrderNo.value}`)
    if (raw) payData.value = JSON.parse(raw)
  } catch (error) {
    payData.value = null
  }

  try {
    const res = await userAPI.getRechargeResult({ pay_order_no: payOrderNo.value })
    order.value = res
    if (res.status === 1) {
      paid.value = true
      clearPayData()
    } else if (res.status === 3 || res.status === 2) {
      closed.value = true
    } else {
      startPolling()
    }
  } catch (error) {
    ElMessage.error('加载支付详情失败')
  } finally {
    pageLoading.value = false
  }
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<style scoped>
.pay-detail-container {
  min-height: 100%;
}

.content {
  max-width: 560px;
  margin: 0 auto;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
}

.result-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 40px 0;
}

.result-icon {
  font-size: 56px;
  color: var(--text-muted);
}

.result-icon.success {
  color: var(--color-success);
}

.result-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}

.result-amount {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-success);
  margin-bottom: 12px;
}

.pay-action {
  margin-top: 24px;
  text-align: center;
}

.qr-box {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.qr-box :deep(svg) {
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  padding: 8px;
}

.qr-tip {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 16px 0 8px;
}

.pay-btn {
  width: 240px;
}

.pay-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 24px;
}

.muted {
  color: var(--text-muted);
}

.tip {
  margin-top: 16px;
  text-align: center;
  font-size: 13px;
}
</style>
