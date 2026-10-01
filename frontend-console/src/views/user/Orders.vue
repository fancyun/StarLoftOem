<template>
  <div class="orders-container">
    <div class="content">
      <div class="card">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Document /></el-icon>
            我的订单
          </h3>
          <el-button @click="loadOrders">刷新</el-button>
        </div>
        <p class="orders-tip">以下为待支付订单，请在有效期内完成支付，超时将自动关闭。</p>

        <el-table :data="orders" style="width: 100%" v-loading="loading">
          <el-table-column prop="pay_order_no" label="订单号" min-width="200" />
          <el-table-column label="用途" width="150">
            <template #default="{ row }">{{ purposeText(row) }}</template>
          </el-table-column>
          <el-table-column label="应付金额" width="120">
            <template #default="{ row }">¥{{ Number(row.total_amount ?? row.amount ?? 0).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column label="剩余有效期" width="140">
            <template #default="{ row }">{{ remainText(row.expire_time) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="180">
            <template #default="{ row }">
              <el-button link type="primary" @click="continuePay(row)">继续支付</el-button>
              <el-button link type="danger" @click="cancelOrder(row)">取消订单</el-button>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无待支付订单" />
          </template>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document } from '@element-plus/icons-vue'
import { userAPI } from '@/api'

const router = useRouter()

const orders = ref<any[]>([])
const loading = ref(false)
// 每秒刷新一次的当前时间戳（Unix 秒），用于计算剩余有效期
const now = ref(Math.floor(Date.now() / 1000))
let timer: ReturnType<typeof setInterval> | null = null

const loadOrders = async () => {
  loading.value = true
  try {
    const res: any = await userAPI.listOrders({ status: 0 })
    orders.value = res.list || []
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const purposeText = (row: any) => {
  if (row.intent === 'recharge') return '余额充值'
  if (row.intent === 'resource_pack') {
    return String(row.product || '').startsWith('sms') ? '短信资源包' : '人脸核验资源包'
  }
  return '支付订单'
}

const remainText = (expireTime: number) => {
  const t = Number(expireTime || 0) - now.value
  if (t <= 0) return '已过期'
  const h = Math.floor(t / 3600)
  const m = Math.floor((t % 3600) / 60)
  const s = t % 60
  if (h > 0) return `${h}小时${m}分`
  return `${m}分${s}秒`
}

const continuePay = (row: any) => {
  router.push(`/payment/${row.pay_order_no}`)
}

const cancelOrder = (row: any) => {
  ElMessageBox.confirm('确认取消该订单？', '取消订单', {
    confirmButtonText: '确定',
    cancelButtonText: '再想想',
    type: 'warning'
  })
    .then(async () => {
      try {
        await userAPI.cancelOrder(row.pay_order_no)
        ElMessage.success('订单已取消')
        await loadOrders()
      } catch (error: any) {
        ElMessage.error(error?.message || '取消订单失败')
      }
    })
    .catch(() => {})
}

onMounted(() => {
  loadOrders()
  timer = setInterval(() => {
    now.value = Math.floor(Date.now() / 1000)
  }, 1000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.orders-container {
  min-height: 100%;
}

.content {
  max-width: 1000px;
  margin: 0 auto;
}

.orders-tip {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0 0 16px;
  padding: 10px 14px;
  background: var(--bg-page);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}
</style>