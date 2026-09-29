<template>
  <div class="admin-bills page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Tickets /></el-icon>
          账单统计
        </h3>
        <div class="head-right">
          <el-input
            v-model="filters.user_id"
            placeholder="用户ID"
            clearable
            style="width: 140px"
            @keyup.enter="onFilterChange"
          />
          <el-select v-model="filters.product" placeholder="产品" clearable style="width: 120px" @change="onFilterChange">
            <el-option label="人脸核验" value="fv" />
            <el-option label="短信" value="sms" />
            <el-option label="实名" value="kyc" />
          </el-select>
          <el-select v-model="filters.bill_type" placeholder="账单类型" clearable style="width: 130px" @change="onFilterChange">
            <el-option label="充值" :value="1" />
            <el-option label="消费" :value="2" />
            <el-option label="退款" :value="3" />
          </el-select>
          <el-select v-model="filters.spend_type" placeholder="花费类型" clearable style="width: 140px" @change="onFilterChange">
            <el-option label="充值入账" value="recharge" />
            <el-option label="余额直接扣费" value="balance" />
            <el-option label="购买资源包" value="pack_purchase" />
            <el-option label="退款" value="refund" />
          </el-select>
          <el-button type="primary" :icon="Refresh" circle @click="loadRecords" />
        </div>
      </div>

      <el-table :data="records" class="table-fill" height="100%" v-loading="loading">
        <el-table-column prop="biz_no" label="业务单号" width="150" />
        <el-table-column prop="user_id" label="用户ID" width="80" />
        <el-table-column prop="product" label="产品" width="80">
          <template #default="{ row }">{{ productLabel(row.product) }}</template>
        </el-table-column>
        <el-table-column prop="service" label="服务" width="100" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag :type="billTypeTag(row.bill_type)" disable-transitions>{{ billTypeLabel(row.bill_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="花费类型" width="130">
          <template #default="{ row }">{{ spendTypeLabel(row.spend_type) }}</template>
        </el-table-column>
        <el-table-column label="计费方式" width="90">
          <template #default="{ row }">{{ payTypeLabel(row.pay_type) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="110">
          <template #default="{ row }">
            <span :class="amountClass(row.bill_type)">{{ amountPrefix(row.bill_type) }}¥{{ Number(row.amount).toFixed(4) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="余额快照" width="150">
          <template #default="{ row }">{{ Number(row.balance_before).toFixed(2) }} → {{ Number(row.balance_after).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="关联" width="150">
          <template #default="{ row }">
            <span v-if="row.ref_type">{{ row.ref_type }}#{{ row.ref_id }}</span>
            <span v-else class="muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160" show-overflow-tooltip />
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadRecords"
        @size-change="loadRecords"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Tickets, Refresh } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])

const filters = reactive<{ user_id: string; product: string; bill_type: string; spend_type: string }>({
  user_id: '',
  product: '',
  bill_type: '',
  spend_type: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const onFilterChange = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getBills({
      user_id: filters.user_id ? Number(filters.user_id) : undefined,
      product: filters.product || undefined,
      bill_type: filters.bill_type ? Number(filters.bill_type) : undefined,
      spend_type: filters.spend_type || undefined,
      page: pagination.page,
      page_size: pagination.pageSize
    })
    records.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载账单失败')
  } finally {
    loading.value = false
  }
}

const productLabel = (p: string) => {
  if (p === 'fv') return '人脸核验'
  if (p === 'sms') return '短信'
  if (p === 'kyc') return '实名'
  return p || '-'
}

const billTypeLabel = (t: number) => {
  if (t === 1) return '充值'
  if (t === 2) return '消费'
  if (t === 3) return '退款'
  return '-'
}

const billTypeTag = (t: number) => {
  if (t === 1) return 'success'
  if (t === 3) return 'danger'
  return 'primary'
}

const spendTypeLabel = (s: string) => {
  const map: Record<string, string> = {
    recharge: '充值入账',
    balance: '余额直接扣费',
    pack_purchase: '购买资源包',
    refund: '退款'
  }
  return map[s] || s || '-'
}

const payTypeLabel = (p: number) => {
  if (p === 0) return '资源包'
  if (p === 1) return '余额'
  if (p === 2) return '支付宝'
  if (p === 3) return '微信支付'
  return '-'
}

const amountPrefix = (t: number) => {
  if (t === 3) return '-'
  if (t === 1) return '+'
  return ''
}

const amountClass = (t: number) => {
  if (t === 1) return 'amount-in'
  if (t === 3) return 'amount-out'
  return 'amount-consume'
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.admin-bills {
  min-height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  gap: 12px;
  flex-wrap: wrap;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.muted {
  color: var(--text-muted);
}

.amount-in {
  color: var(--color-success);
  font-weight: 600;
}

.amount-out {
  color: var(--color-danger);
  font-weight: 600;
}

.amount-consume {
  color: var(--text-primary);
  font-weight: 600;
}
</style>
