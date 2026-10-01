<template>
  <div class="admin-payments page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Wallet /></el-icon>
        支付记录
      </h3>

      <div class="filter-bar">
        <el-input
          v-model="filters.payOrderNo"
          placeholder="支付流水号"
          style="width: 200px"
          clearable
          @keyup.enter="onFilterChange"
        />
        <el-select v-model="filters.channel" placeholder="支付渠道" clearable style="width: 150px" @change="onFilterChange">
          <el-option label="余额支付" value="balance" />
          <el-option label="支付宝" value="alipay" />
          <el-option label="微信支付" value="wechat" />
          <el-option label="人工支付" value="manual" />
        </el-select>
        <el-select v-model="filters.status" placeholder="订单状态" clearable style="width: 150px" @change="onFilterChange">
          <el-option label="待支付" :value="0" />
          <el-option label="已支付" :value="1" />
          <el-option label="已退款" :value="2" />
          <el-option label="已关闭" :value="3" />
        </el-select>
        <el-button type="primary" icon="Search" @click="onFilterChange">搜索</el-button>
      </div>

      <el-table :data="orders" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="pay_order_no" label="支付流水号" width="180" />
        <el-table-column prop="user_phone" label="用户手机号" width="140" />
        <el-table-column prop="channel" label="支付渠道" width="110">
          <template #default="{ row }">
            <el-tag>{{ getChannelText(row.channel) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="支付金额" width="110">
          <template #default="{ row }">¥{{ row.amount }}</template>
        </el-table-column>
        <el-table-column label="用途" width="120">
          <template #default="{ row }">{{ row.intent === 'recharge' ? '余额充值' : '购买资源包' }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getPayStatusType(row.status)">{{ getPayStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="paid_at" label="支付时间" width="180">
          <template #default="{ row }">{{ row.paid_at ? formatDateTime(row.paid_at) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="bank_serial_no" label="银行流水单号" min-width="200">
          <template #default="{ row }">{{ row.bank_serial_no || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button size="small" @click="viewDetail(row)">详情</el-button>
            <el-button v-if="row.status === 0 && canWrite" size="small" type="warning" @click="queryResult(row)">查询结果</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadOrders"
        @size-change="onFilterChange"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>

    <!-- 支付记录详情 -->
    <el-dialog v-model="detailVisible" title="支付记录详情" width="640px" top="6vh">
      <el-descriptions :column="2" border v-if="detail">
        <el-descriptions-item label="支付流水号" :span="2">{{ detail.pay_order_no || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用户ID">{{ detail.user_id }}</el-descriptions-item>
        <el-descriptions-item label="用户手机号">{{ detail.user_phone || '-' }}</el-descriptions-item>
        <el-descriptions-item label="支付渠道">{{ getChannelText(detail.channel) }}</el-descriptions-item>
        <el-descriptions-item label="支付金额">¥{{ detail.amount }}</el-descriptions-item>
        <el-descriptions-item label="余额支付">¥{{ detail.balance_amount }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="getPayStatusType(detail.status)">{{ getPayStatusText(detail.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="用途">{{ detail.intent === 'recharge' ? '余额充值' : '购买资源包' }}</el-descriptions-item>
        <el-descriptions-item label="退款状态">{{ getRefundStatusText(detail.refund_status) }}</el-descriptions-item>
        <el-descriptions-item label="退款金额">¥{{ detail.refund_amount }}</el-descriptions-item>
        <el-descriptions-item label="渠道交易号" :span="2">{{ detail.channel_trade_no || '-' }}</el-descriptions-item>
        <el-descriptions-item label="银行流水单号" :span="2">{{ detail.bank_serial_no || '-' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatDateTime(detail.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="支付时间">{{ detail.paid_at ? formatDateTime(detail.paid_at) : '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Wallet } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 sys.finance.write 时隐藏「查询结果」按钮（该操作会写库）
const canWrite = computed(() => adminStore.has('sys.finance.write'))

const loading = ref(false)
const orders = ref([])

const filters = reactive({
  payOrderNo: '',
  channel: '',
  status: null as number | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const detailVisible = ref(false)
const detail = ref<any>(null)

onMounted(() => {
  loadOrders()
})

const onFilterChange = () => {
  pagination.page = 1
  loadOrders()
}

const loadOrders = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getPayments({
      page: pagination.page,
      page_size: pagination.pageSize,
      pay_order_no: filters.payOrderNo || undefined,
      channel: filters.channel || undefined,
      status: filters.status ?? undefined
    })
    orders.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载支付记录失败')
  } finally {
    loading.value = false
  }
}

const getChannelText = (channel: string) => {
  const map: Record<string, string> = { balance: '余额支付', alipay: '支付宝', wechat: '微信支付', manual: '人工支付' }
  return map[channel] || channel || '未选择'
}

const getPayStatusType = (status: number) => {
  const map: Record<number, string> = { 0: 'warning', 1: 'success', 2: 'info', 3: 'info' }
  return map[status] || 'info'
}

const getPayStatusText = (status: number) => {
  const map: Record<number, string> = { 0: '待支付', 1: '已支付', 2: '已退款', 3: '已关闭' }
  return map[status] || '未知'
}

const getRefundStatusText = (status: number) => {
  const map: Record<number, string> = { 0: '未退款', 1: '部分退款', 2: '全额退款' }
  return map[status] ?? '未退款'
}

const viewDetail = (row: any) => {
  detail.value = row
  detailVisible.value = true
}

// 手动查询支付结果：向渠道查询待支付单真实状态并落地（渠道已支付→补账/发放，已关闭→关闭本地单）。
// 查询不产生写操作副作用，属非敏感操作，无需二次确认
const queryResult = async (row: any) => {
  try {
    const response: any = await adminAPI.queryPaymentResult(row.id)
    ElMessage.success(response?.result || '查询完成')
    loadOrders()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败')
  }
}
</script>

<style scoped>
.admin-payments {
  min-height: 100%;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
}
</style>
