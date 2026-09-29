<template>
  <div class="balance-container">
    <div class="content">
      <div class="card" v-loading="pageLoading">
        <h3 class="section-title">
          <el-icon><Wallet /></el-icon>
          余额管理
        </h3>
        <div class="balance-display">
          <span class="balance-label">当前余额</span>
          <span class="balance-amount">¥{{ userInfo.balance }}</span>
        </div>

        <el-form :model="rechargeForm" class="recharge-form">
          <el-form-item label="充值金额">
            <el-input
              v-model="rechargeForm.amount"
              placeholder="请输入充值金额"
              type="number"
            >
              <template #append>元</template>
            </el-input>
          </el-form-item>
          <el-form-item label="支付方式">
            <el-radio-group v-model="rechargeForm.channel">
              <el-radio v-for="ch in payChannelOptions" :key="ch.value" :value="ch.value">{{ ch.label }}</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-button type="primary" @click="handleRecharge" :loading="rechargeLoading">
            充值
          </el-button>
        </el-form>

      </div>

      <div class="card">
        <h3 class="section-title">
          <el-icon><RefreshLeft /></el-icon>
          提现
        </h3>
        <div class="withdraw-display">
          <div>
            <span class="balance-label">可提现金额</span>
            <span class="balance-amount">¥{{ withdrawable.toFixed(2) }}</span>
          </div>
          <p class="withdraw-tip">
            提现将按充值支付订单原路退回至支付宝/微信；支持部分提现，金额不足时自动拆分多个订单退款，退款后该订单剩余可退金额自动更新。
          </p>
        </div>
        <el-form :model="withdrawForm" class="recharge-form">
          <el-form-item label="提现金额">
            <el-input
              v-model="withdrawForm.amount"
              placeholder="请输入提现金额"
              type="number"
            >
              <template #append>元</template>
            </el-input>
          </el-form-item>
          <el-button type="primary" @click="handleWithdraw" :loading="withdrawLoading">
            提现
          </el-button>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { RefreshLeft } from '@element-plus/icons-vue'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { PAY_CHANNEL_OPTIONS, loadEnabledPayChannels } from '@/utils/payment'

const router = useRouter()
const userStore = useUserStore()

const userInfo = ref<any>({})
const rechargeLoading = ref(false)
const pageLoading = ref(true)

// 提现
const withdrawForm = reactive({ amount: '' })
const withdrawLoading = ref(false)
const withdrawable = ref(0)

// 移动端走微信 H5 跳转，PC 端走 Native 扫码
const isMobile = () => {
  return (
    /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
    (navigator.maxTouchPoints > 1 && window.innerWidth < 768)
  )
}

// 已启用的在线支付渠道（未配置凭据的渠道不展示）
const enabledChannels = ref<string[]>(['alipay', 'wechat'])
const payChannelOptions = computed(() => PAY_CHANNEL_OPTIONS.filter((ch) => enabledChannels.value.includes(ch.value)))

const rechargeForm = reactive({
  amount: '',
  channel: 'alipay'
})

const handleRecharge = async () => {
  if (!rechargeForm.amount || Number(rechargeForm.amount) <= 0) {
    ElMessage.warning('请输入正确的充值金额')
    return
  }
  rechargeLoading.value = true
  try {
    const res = await userAPI.createRecharge({
      amount: Number(rechargeForm.amount),
      channel: rechargeForm.channel,
      scene: rechargeForm.channel === 'wechat' && !isMobile() ? 'native' : 'h5'
    })

    // 一次性支付信息（支付链接/二维码）存 sessionStorage，供支付详情页展示
    sessionStorage.setItem(
      `starloft_pay_${res.pay_order_no}`,
      JSON.stringify({ ...res, pay_purpose: 'recharge', return_path: '/balance' })
    )

    // 跳转到支付详情页，避免支付完成后只能回到充值页面
    router.push(`/payment/${res.pay_order_no}`)
  } catch (error: any) {
    ElMessage.error(error?.message || '发起充值失败，请稍后重试')
  } finally {
    rechargeLoading.value = false
  }
}

const loadData = async () => {
  try {
    const profile = await userAPI.getProfile()
    userInfo.value = profile
    userStore.setUserInfo(profile)
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
  }
}

// 加载可提现金额（已支付充值订单剩余可退款合计）
const loadWithdrawable = async () => {
  try {
    const res: any = await userAPI.getRefundableOrders()
    withdrawable.value = Number(res?.total || 0)
  } catch (error) {
    console.error(error)
  }
}

const handleWithdraw = async () => {
  if (!withdrawForm.amount || Number(withdrawForm.amount) <= 0) {
    ElMessage.warning('请输入正确的提现金额')
    return
  }
  if (Number(withdrawForm.amount) > withdrawable.value) {
    ElMessage.warning(`可提现金额为 ¥${withdrawable.value.toFixed(2)}`)
    return
  }
  withdrawLoading.value = true
  try {
    await userAPI.withdraw({ amount: Number(withdrawForm.amount) })
    ElMessage.success('提现成功，款项将原路退回')
    withdrawForm.amount = ''
  } catch (error: any) {
    // 失败（含部分成功）后刷新余额与可提现金额
    ElMessage.error(error?.message || '提现失败，请稍后重试')
  } finally {
    withdrawLoading.value = false
    await Promise.all([loadData(), loadWithdrawable()])
  }
}

onMounted(async () => {
  loadData()
  loadWithdrawable()
  enabledChannels.value = await loadEnabledPayChannels()
  if (!enabledChannels.value.includes(rechargeForm.channel)) {
    rechargeForm.channel = enabledChannels.value[0] || 'alipay'
  }
})
</script>

<style scoped>
.balance-container {
  min-height: 100%;
}

.content {
  max-width: 700px;
  margin: 0 auto;
}

.balance-display {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px;
  background: linear-gradient(135deg, var(--color-primary-light) 0%, #D6E8FF 100%);
  border-radius: var(--radius-md);
  margin-bottom: 24px;
}

.balance-label {
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 8px;
}

.balance-amount {
  color: var(--text-primary);
  font-size: 32px;
  font-weight: 700;
}

.recharge-form {
  margin-top: 8px;
}

.withdraw-display {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  margin-bottom: 8px;
}

.withdraw-tip {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
}

</style>
