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
          <div class="result-title">{{ successTitle }}</div>
          <div class="result-amount">+¥{{ totalAmount.toFixed(2) }}</div>
          <el-button type="primary" @click="goBack">{{ backText }}</el-button>
        </div>

        <!-- 已关闭/已退款 -->
        <div v-else-if="closed" class="result-box">
          <el-icon class="result-icon"><CircleClose /></el-icon>
          <div class="result-title">{{ statusText }}</div>
          <el-button type="primary" @click="goBack">重新下单</el-button>
        </div>

        <!-- 待支付 -->
        <template v-else>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="支付单号">{{ payOrderNo }}</el-descriptions-item>
            <el-descriptions-item label="订单用途">{{ purposeText }}</el-descriptions-item>
            <el-descriptions-item label="应付总额">¥{{ totalAmount.toFixed(2) }}</el-descriptions-item>
            <el-descriptions-item label="可用余额">¥{{ balanceAvailable.toFixed(2) }}</el-descriptions-item>
          </el-descriptions>

          <!-- 订单已绑定在线渠道：渲染二维码 / 支付链接（刷新后由订单 pay_info 恢复） -->
          <div v-if="activePayInfo" class="pay-action">
            <div v-if="activePayInfo.pay_type === 'native' && activePayInfo.code_url" class="qr-box">
              <qrcode-vue :value="activePayInfo.code_url" :size="190" level="H" />
              <p class="qr-tip">请使用微信扫码完成支付</p>
            </div>
            <el-button
              v-else-if="activePayInfo.pay_type === 'url' && activePayInfo.pay_url"
              type="primary"
              size="large"
              class="pay-btn"
              @click="openPayUrl"
            >
              打开支付宝
            </el-button>
            <el-button
              v-else-if="activePayInfo.pay_type === 'h5' && activePayInfo.h5_url"
              type="primary"
              size="large"
              class="pay-btn"
              @click="openH5"
            >
              前往支付
            </el-button>
          </div>

          <!-- 选择支付方式 -->
          <div v-else class="method-box">
            <!-- 余额全额支付（仅资源包订单提供） -->
            <div v-if="canUseBalance" class="balance-method">
              <el-button
                type="primary"
                size="large"
                :disabled="balanceInsufficient"
                :loading="paying === 'balance'"
                @click="payWith('balance')"
              >
                余额支付
              </el-button>
              <span v-if="balanceInsufficient" class="muted">可用余额不足，请选择在线支付</span>
            </div>

            <!-- 在线渠道（可选余额抵扣差额） -->
            <template v-if="onlineMethods.length">
              <div v-if="showBalanceOption" class="balance-opt">
                <el-checkbox v-model="useBalance">使用余额抵扣</el-checkbox>
                <span class="deduct-text">
                  <template v-if="useBalance">
                    本次抵扣 ¥{{ balanceDeduct.toFixed(2) }} + 在线支付 ¥{{ onlinePayAmount.toFixed(2) }}
                  </template>
                  <template v-else>在线支付 ¥{{ totalAmount.toFixed(2) }}</template>
                </span>
              </div>
              <div class="online-methods">
                <el-button
                  v-for="m in onlineMethods"
                  :key="m"
                  :loading="paying === m"
                  @click="payWith(m)"
                >
                  {{ channelLabel(m) }}
                </el-button>
              </div>
            </template>
          </div>

          <div class="pay-actions">
            <el-button type="primary" :loading="checking" @click="checkPaid">查询支付结果</el-button>
            <el-button @click="handleCancel">取消订单</el-button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
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
// 正在发起支付的渠道
const paying = ref('')

// 本次下单返回的在线支付信息（刷新后由订单 pay_info 恢复）
const payResult = ref<any>(null)
// 在线支付时是否先用余额抵扣（默认勾选）
const useBalance = ref(true)

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollCount = 0
const POLL_MAX = 120 // 最多轮询约5分钟

const intent = computed(() => order.value.intent || '')
const totalAmount = computed(() => Number(order.value.total_amount ?? order.value.amount ?? 0))
const balanceAvailable = computed(() => Number(order.value.balance_available || 0))
const payMethods = computed<string[]>(() =>
  Array.isArray(order.value.pay_methods) ? order.value.pay_methods : []
)
const onlineMethods = computed(() => payMethods.value.filter((m) => m === 'alipay' || m === 'wechat'))
// 余额全额支付仅资源包订单提供
const canUseBalance = computed(() => intent.value === 'resource_pack' && payMethods.value.includes('balance'))
const balanceInsufficient = computed(() => balanceAvailable.value < totalAmount.value)
// 在线支付可用余额抵扣（余额仅在资源包订单中参与）
const showBalanceOption = computed(() => intent.value === 'resource_pack' && onlineMethods.value.length > 0)
const balanceDeduct = computed(() => Math.min(balanceAvailable.value, totalAmount.value))
const onlinePayAmount = computed(() => Math.max(totalAmount.value - balanceDeduct.value, 0))

const activePayInfo = computed(() => order.value.pay_info || payResult.value)

const channelLabel = (method: string) => PAY_CHANNEL_LABELS[method] || method

const successTitle = computed(() => (intent.value === 'recharge' ? '充值成功' : '购买成功'))

const purposeText = computed(() => {
  if (intent.value === 'recharge') return '余额充值'
  if (intent.value === 'resource_pack') return '购买资源包'
  return '支付订单'
})

// 支付成功后返回来源页（充值→余额管理；资源包→对应产品资源包页）
const backPath = computed(() => {
  if (intent.value === 'recharge') return '/balance'
  const product = String(order.value.product || '')
  return product.startsWith('sms') ? '/sms/packs' : '/fv/packs'
})

const backText = computed(() => {
  if (intent.value === 'recharge') return '返回余额管理'
  const product = String(order.value.product || '')
  return product.startsWith('sms') ? '返回短信资源包' : '返回资源包'
})

const statusText = computed(() => {
  if (order.value.status === 3) return '订单已关闭'
  if (order.value.status === 2) return '订单已退款'
  return '订单已结束'
})

const goBack = () => {
  router.push(backPath.value)
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
  pollTimer = setInterval(() => {
    pollCount++
    if (pollCount > POLL_MAX) {
      stopPolling()
      return
    }
    queryResult()
  }, 2500)
}

// 应用订单状态：已支付/已关闭时终止轮询并切换页面态
const applyOrder = (res: any) => {
  order.value = res
  if (res.status === 1) {
    stopPolling()
    paid.value = true
    payResult.value = null
  } else if (res.status === 2 || res.status === 3) {
    stopPolling()
    closed.value = true
  }
}

const queryResult = async () => {
  try {
    const res = await userAPI.getOrder(payOrderNo.value)
    applyOrder(res)
  } catch (error) {
    // 查询失败忽略，轮询继续
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

const isMobile = /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent)

const payWith = async (method: 'balance' | 'alipay' | 'wechat') => {
  paying.value = method
  try {
    const data: { method: 'balance' | 'alipay' | 'wechat'; scene?: 'native' | 'h5'; use_balance?: boolean } = {
      method
    }
    if (method !== 'balance') {
      // 微信非移动端走 Native 二维码，其余走 h5
      data.scene = method === 'wechat' && !isMobile ? 'native' : 'h5'
      data.use_balance = showBalanceOption.value ? useBalance.value : false
    }
    const res: any = await userAPI.payOrder(payOrderNo.value, data)
    if (res.fully_paid) {
      // 余额（或抵扣后差额）已付清，直接进入成功态
      await queryResult()
      if (!paid.value) {
        stopPolling()
        paid.value = true
      }
      return
    }
    payResult.value = res
    // h5 支付直接跳转渠道收银台
    if (res.pay_type === 'h5' && res.h5_url) {
      window.location.href = res.h5_url
    }
  } catch (error: any) {
    ElMessage.error(error?.message || '发起支付失败')
  } finally {
    paying.value = ''
  }
}

const openPayUrl = () => {
  if (activePayInfo.value?.pay_url) window.open(activePayInfo.value.pay_url, '_blank')
}

const openH5 = () => {
  if (activePayInfo.value?.h5_url) window.location.href = activePayInfo.value.h5_url
}

const handleCancel = () => {
  ElMessageBox.confirm('确认取消该订单？', '取消订单', {
    confirmButtonText: '确定',
    cancelButtonText: '再想想',
    type: 'warning'
  })
    .then(async () => {
      try {
        await userAPI.cancelOrder(payOrderNo.value)
        ElMessage.success('订单已取消')
        stopPolling()
        goBack()
      } catch (error: any) {
        ElMessage.error(error?.message || '取消订单失败')
      }
    })
    .catch(() => {})
}

onMounted(async () => {
  try {
    const res = await userAPI.getOrder(payOrderNo.value)
    applyOrder(res)
    if (res.status === 0) startPolling()
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

.method-box {
  margin-top: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.balance-method {
  display: flex;
  align-items: center;
  gap: 12px;
}

.balance-opt {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.deduct-text {
  font-size: 13px;
  color: var(--text-secondary);
}

.online-methods {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
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