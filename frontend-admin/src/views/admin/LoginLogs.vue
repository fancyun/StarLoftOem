<template>
  <div class="admin-login-logs page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Document /></el-icon>
        登录日志
      </h3>

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="用户登录" name="user" />
        <el-tab-pane label="管理员登录" name="admin" />
      </el-tabs>

      <div class="filter-bar">
        <el-input
          v-model="filters.keyword"
          placeholder="搜索账号/IP"
          style="width: 200px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #append>
            <el-button icon="Search" @click="handleSearch" />
          </template>
        </el-input>

        <el-select v-model="filters.status" placeholder="登录状态" clearable style="width: 140px" @change="handleSearch">
          <el-option label="成功" :value="1" />
          <el-option label="失败" :value="0" />
        </el-select>

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

      <el-table :data="list" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column v-if="activeTab === 'user'" label="用户ID" width="90">
          <template #default="{ row }">{{ row.user_id ?? '-' }}</template>
        </el-table-column>
        <el-table-column v-else label="管理员ID" width="100">
          <template #default="{ row }">{{ row.admin_id ?? '-' }}</template>
        </el-table-column>
        <el-table-column v-if="activeTab === 'user'" prop="account" label="账号" min-width="140" show-overflow-tooltip />
        <el-table-column v-else prop="username" label="账号" min-width="140" show-overflow-tooltip />
        <el-table-column v-if="activeTab === 'user'" label="登录方式" width="110">
          <template #default="{ row }">
            <el-tag>{{ loginTypeText(row.login_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="150" show-overflow-tooltip />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="fail_reason" label="失败原因" min-width="160" show-overflow-tooltip>
          <template #default="{ row }">{{ row.fail_reason || '-' }}</template>
        </el-table-column>
        <el-table-column prop="user_agent" label="User-Agent" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">{{ row.user_agent || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="登录时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadLogs"
        @size-change="handleSearch"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Document } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const activeTab = ref<'user' | 'admin'>('user')
const loading = ref(false)
const list = ref<any[]>([])

const filters = reactive({
  keyword: '',
  status: null as number | null,
  dateRange: null as [string, string] | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 登录方式展示：password-密码 sms_code-短信 wechat-微信
const loginTypeText = (type: string) => {
  const map: Record<string, string> = { password: '密码', sms_code: '短信', wechat: '微信' }
  return map[type] || type || '-'
}

onMounted(() => {
  loadLogs()
})

const handleTabChange = () => {
  pagination.page = 1
  list.value = []
  pagination.total = 0
  loadLogs()
}

const handleSearch = () => {
  pagination.page = 1
  loadLogs()
}

const loadLogs = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      keyword: filters.keyword || undefined,
      status: filters.status ?? undefined,
      start_date: filters.dateRange?.[0],
      end_date: filters.dateRange?.[1]
    }
    const res: any =
      activeTab.value === 'user'
        ? await adminAPI.getUserLoginLogs(params)
        : await adminAPI.getAdminLoginLogs(params)
    list.value = res.list || []
    pagination.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-login-logs {
  min-height: 100%;
}
</style>