<template>
  <div class="admin-promotion-withdraws page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Wallet /></el-icon>
          提现审核
        </h3>
        <div class="head-right">
          <el-input
            v-model="filters.referrerId"
            placeholder="推广商 ID"
            style="width: 160px"
            clearable
            @keyup.enter="handleSearch"
            @clear="handleSearch"
          />
          <el-select v-model="filters.status" style="width: 160px" @change="handleSearch">
            <el-option label="全部" :value="-1" />
            <el-option label="待审核" :value="0" />
            <el-option label="已通过待打款" :value="1" />
            <el-option label="已完成" :value="2" />
            <el-option label="已驳回" :value="3" />
          </el-select>
        </div>
      </div>

      <el-table :data="list" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="申请 ID" width="90" />
        <el-table-column prop="referrer_id" label="推广商 ID" width="110" />
        <el-table-column label="提现方式" width="110">
          <template #default="{ row }">{{ channelLabel(row.channel) }}</template>
        </el-table-column>
        <el-table-column label="提现金额" width="110">
          <template #default="{ row }">¥{{ row.amount }}</template>
        </el-table-column>
        <el-table-column label="手续费" width="100">
          <template #default="{ row }">¥{{ row.fee }}</template>
        </el-table-column>
        <el-table-column label="实际到账" width="110">
          <template #default="{ row }">¥{{ row.actual_amount }}</template>
        </el-table-column>
        <el-table-column prop="payee_info" label="收款信息" min-width="180">
          <template #default="{ row }">{{ row.payee_info || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reject_reason" label="驳回原因" min-width="140">
          <template #default="{ row }">{{ row.reject_reason || '-' }}</template>
        </el-table-column>
        <el-table-column label="申请时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="200">
          <template #default="{ row }">
            <template v-if="canWrite && row.status === 0">
              <el-button link type="success" @click="handleApprove(row)">通过</el-button>
              <el-button link type="danger" @click="handleReject(row)">驳回</el-button>
            </template>
            <el-button v-else-if="canWrite && row.status === 1" link type="primary" @click="handleMarkPaid(row)">标记已打款</el-button>
            <span v-else>-</span>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadList"
        @size-change="loadList"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Wallet } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 sys.aff.write 时隐藏审核/打款按钮
const canWrite = computed(() => adminStore.has('sys.aff.write'))

interface PromotionWithdraw {
  id: number
  referrer_id: number
  channel: string // 提现方式：balance-提现到余额（线下渠道后续扩展）
  amount: number
  fee_rate: number
  fee: number
  actual_amount: number
  status: number
  payee_info: string
  reject_reason: string
  created_at: string
}

// 提现方式文案（当前仅 balance-提现到余额）
const CHANNEL_LABELS: Record<string, string> = { balance: '余额' }
const channelLabel = (channel: string) => CHANNEL_LABELS[channel] || channel || '-'

// 状态枚举：0-待审核 1-已通过待打款 2-已完成 3-已驳回
const statusText = (status: number) => {
  const map: Record<number, string> = { 0: '待审核', 1: '已通过待打款', 2: '已完成', 3: '已驳回' }
  return map[status] || '-'
}

const statusTag = (status: number) => {
  const map: Record<number, 'warning' | 'success' | 'danger' | 'info'> = {
    0: 'warning',
    1: 'success',
    2: 'info',
    3: 'danger'
  }
  return map[status] || 'info'
}

const loading = ref(false)
const list = ref<PromotionWithdraw[]>([])
const filters = reactive<{ referrerId: string; status: number }>({ referrerId: '', status: -1 })

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadList()
})

const loadList = async () => {
  loading.value = true
  try {
    const referrerId = filters.referrerId.trim() === '' ? undefined : Number(filters.referrerId.trim())
    const res: any = await adminAPI.listPromotionWithdraws({
      // 提现仅用户型推广：按推广商 user.id 过滤需同时传 referrer_type
      referrer_type: referrerId && !Number.isNaN(referrerId) ? 'user' : undefined,
      referrer_id: referrerId && !Number.isNaN(referrerId) ? referrerId : undefined,
      status: filters.status >= 0 ? filters.status : undefined,
      page: pagination.page,
      page_size: pagination.pageSize
    })
    list.value = res.list || []
    pagination.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadList()
}

const handleApprove = async (row: PromotionWithdraw) => {
  try {
    await ElMessageBox.confirm(`确认通过提现申请 #${row.id}（实际打款 ¥${row.actual_amount}）？`, '通过审核', {
      confirmButtonText: '通过',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await adminAPI.reviewPromotionWithdraw(row.id, { approve: true })
    ElMessage.success('已通过')
    loadList()
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}

const handleReject = (row: PromotionWithdraw) => {
  ElMessageBox.prompt(`请输入驳回提现申请 #${row.id} 的原因`, '驳回申请', {
    confirmButtonText: '驳回',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '驳回原因不能为空'
  })
    .then(async ({ value }) => {
      await adminAPI.reviewPromotionWithdraw(row.id, { approve: false, reject_reason: value })
      ElMessage.success('已驳回')
      loadList()
    })
    .catch(() => {})
}

const handleMarkPaid = async (row: PromotionWithdraw) => {
  try {
    await ElMessageBox.confirm(`确认提现申请 #${row.id} 已线下打款？`, '标记已打款', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await adminAPI.markPromotionWithdrawPaid(row.id)
    ElMessage.success('已标记打款')
    loadList()
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}
</script>

<style scoped>
.admin-promotion-withdraws {
  min-height: 100%;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>