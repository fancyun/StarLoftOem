<template>
  <div class="admin-promoters page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Share /></el-icon>
        推广归属
      </h3>

      <div class="filter-bar">
        <el-select v-model="filters.referrerType" placeholder="类型" clearable style="width: 160px" @change="handleSearch">
          <el-option label="用户推广" value="user" />
          <el-option label="员工销售" value="staff" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="推广码/手机号/名称"
          style="width: 220px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #append>
            <el-button icon="Search" @click="handleSearch" />
          </template>
        </el-input>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="aff_code" label="推广码" width="160">
          <template #default="{ row }">
            <span class="mono">{{ row.aff_code || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="110">
          <template #default="{ row }">
            <el-tag :type="row.referrer_type === 'user' ? 'primary' : 'warning'">
              {{ row.referrer_type === 'user' ? '用户推广' : '员工销售' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" width="140">
          <template #default="{ row }">{{ row.phone || '-' }}</template>
        </el-table-column>
        <el-table-column prop="sub_count" label="下级数" width="90" />
        <el-table-column label="累计提成" width="130">
          <template #default="{ row }">
            <span :class="{ negative: row.commission_total < 0 }">¥{{ row.commission_total }}</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="100">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="goDetail(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadRecords"
        @size-change="handleSearch"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Share } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

const loading = ref(false)
const records = ref<any[]>([])

const filters = reactive({
  referrerType: '',
  keyword: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadRecords()
})

const handleSearch = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getPromoters({
      page: pagination.page,
      page_size: pagination.pageSize,
      referrer_type: filters.referrerType || undefined,
      keyword: filters.keyword || undefined
    })
    records.value = res.list || []
    pagination.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

const goDetail = (row: any) => {
  router.push(`/sys/promoters/${row.referrer_type}/${row.referrer_id}`)
}
</script>

<style scoped>
.admin-promoters {
  min-height: 100%;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.negative {
  color: var(--color-danger);
}
</style>