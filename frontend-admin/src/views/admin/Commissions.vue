<template>
  <div class="admin-commissions page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Wallet /></el-icon>
          提成记录
        </h3>
        <div class="filter-bar">
          <el-input
            v-model="filters.keyword"
            placeholder="推广方 / 来源用户"
            style="width: 180px"
            clearable
            @keyup.enter="handleSearch"
            @clear="handleSearch"
          />
          <el-select
            v-model="filters.referrer_type"
            placeholder="推广类型"
            clearable
            style="width: 130px"
            @change="handleSearch"
          >
            <el-option label="用户型" value="user" />
            <el-option label="员工型" value="staff" />
          </el-select>
          <el-select
            v-model="filters.biz_type"
            placeholder="业务类型"
            clearable
            style="width: 150px"
            @change="handleSearch"
          >
            <el-option label="购买资源包" value="pack_purchase" />
            <el-option label="短信发送" value="sms_send" />
            <el-option label="人脸核验" value="fv_auth" />
            <el-option label="退款冲回" value="refund" />
          </el-select>
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 260px"
            @change="handleSearch"
          />
          <el-button @click="handleReset">重置</el-button>
        </div>
      </div>

      <el-table :data="list" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.referrer_type === 'staff' ? 'warning' : 'primary'" disable-transitions>
              {{ referrerTypeLabel(row.referrer_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="推广方" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.referrer_name || `#${row.referrer_id}` }}</template>
        </el-table-column>
        <el-table-column label="来源用户" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.username || `#${row.user_id}` }}</template>
        </el-table-column>
        <el-table-column label="业务类型" width="120">
          <template #default="{ row }">{{ settlementBizLabel(row.biz_type) }}</template>
        </el-table-column>
        <el-table-column label="金额" width="130">
          <template #default="{ row }">
            <span class="amount">¥{{ Number(row.amount).toFixed(4) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
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
import { ref, reactive, onMounted } from 'vue'
import { Wallet } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime, settlementBizLabel } from '@/utils/format'

interface CommissionRecord {
  id: number
  referrer_type: string
  referrer_id: number
  referrer_name: string
  user_id: number
  username: string
  amount: number
  biz_type: string
  ref_type: string
  ref_id: number
  remark: string
  created_at: string
}

const loading = ref(false)
const list = ref<CommissionRecord[]>([])
const dateRange = ref<[string, string] | null>(null)

const filters = reactive<{ referrer_type: string; biz_type: string; keyword: string }>({
  referrer_type: '',
  biz_type: '',
  keyword: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 推广类型文案：user-用户型 staff-员工型
const referrerTypeLabel = (t: string) => {
  if (t === 'user') return '用户型'
  if (t === 'staff') return '员工型'
  return '-'
}

const loadList = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.listCommissions({
      referrer_type: filters.referrer_type || undefined,
      biz_type: filters.biz_type || undefined,
      keyword: filters.keyword.trim() || undefined,
      start_date: dateRange.value?.[0] || undefined,
      end_date: dateRange.value?.[1] || undefined,
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

const handleReset = () => {
  filters.referrer_type = ''
  filters.biz_type = ''
  filters.keyword = ''
  dateRange.value = null
  handleSearch()
}

onMounted(() => {
  loadList()
})
</script>

<style scoped>
.admin-commissions {
  min-height: 100%;
}

.filter-bar {
  flex-wrap: wrap;
}

.amount {
  color: var(--color-success);
  font-weight: 600;
}
</style>