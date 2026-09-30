<template>
  <div class="admin-api-keys page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Key /></el-icon>
        API 密钥
      </h3>

      <div class="filter-bar">
        <el-input
          v-model="filters.userId"
          placeholder="用户ID"
          style="width: 140px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />
        <el-input
          v-model="filters.keyword"
          placeholder="API Key/名称"
          style="width: 220px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #append>
            <el-button icon="Search" @click="handleSearch" />
          </template>
        </el-input>
        <el-date-picker
          v-model="filters.dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 260px"
          @change="handleSearch"
        />
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="phone" label="手机号" width="140">
          <template #default="{ row }">{{ row.phone || '-' }}</template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="120">
          <template #default="{ row }">{{ row.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.name || '-' }}</template>
        </el-table-column>
        <el-table-column label="API Key" min-width="260">
          <template #default="{ row }">
            <span class="mono-key" :title="row.api_key" @click="copyKey(row.api_key)">{{ row.api_key }}</span>
          </template>
        </el-table-column>
        <el-table-column label="权限范围" min-width="180">
          <template #default="{ row }">
            <el-tag v-if="row.permission === 'all'">全部端点</el-tag>
            <template v-else>
              <el-tag v-for="scope in permissionScopes(row.permission)" :key="scope" style="margin-right: 4px">
                {{ scope }}
              </el-tag>
              <span v-if="!permissionScopes(row.permission).length">-</span>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
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
import { ElMessage } from 'element-plus'
import { Key } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])

const filters = reactive({
  userId: '',
  keyword: '',
  dateRange: null as [string, string] | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 权限范围：all 为全部端点，否则按逗号拆分为多个端点
const permissionScopes = (permission: string): string[] => {
  if (!permission || permission === 'all') return []
  return permission.split(',').map((s) => s.trim()).filter(Boolean)
}

const copyKey = async (value: string) => {
  if (!value) return
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(value)
    } else {
      const el = document.createElement('textarea')
      el.value = value
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}

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
    const res: any = await adminAPI.getAdminApiKeys({
      page: pagination.page,
      page_size: pagination.pageSize,
      user_id: filters.userId || undefined,
      keyword: filters.keyword || undefined,
      start_date: filters.dateRange?.[0],
      end_date: filters.dateRange?.[1]
    })
    records.value = res.list || []
    pagination.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-api-keys {
  min-height: 100%;
}

.mono-key {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  cursor: pointer;
  color: var(--color-primary);
  word-break: break-all;
}
</style>