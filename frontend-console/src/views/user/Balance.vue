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
          <el-button type="primary" @click="handleRecharge" :loading="rechargeLoading">
            创建订单
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
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { RefreshLeft } from '@element-plus/icons-vue'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const userStore = useUserStore()

const userInfo = ref<any>({})
const rechargeLoading = ref(false)
const pageLoading = ref(true)

// 提现
const withdrawForm = reactive({ amount: '' })
const withdrawLoading = ref(false)
const withdrawable = ref(0)

const rechargeForm = reactive({
  amount: ''
})

const handleRecharge = async () => {
  if (!rechargeForm.amount || Number(rechargeForm.amount) <= 0) {
    ElMessage.warning('请输入正确的充值金额')
    return
  }
  rechargeLoading.value = true
  try {
    const res: any = await userAPI.createOrder({
      intent: 'recharge',
      amount: Number(rechargeForm.amount)
    })
    // 建单成功后跳转统一支付页选择支付方式
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

onMounted(() => {
  loadData()
  loadWithdrawable()
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
