<template>
  <div class="admin-kyc page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><User /></el-icon>
          账户个人实名记录
        </h3>
        <el-select v-model="filters.status" placeholder="认证状态" clearable style="width: 150px" @change="onFilterChange">
          <el-option label="待认证" :value="0" />
          <el-option label="认证中" :value="1" />
          <el-option label="认证成功" :value="2" />
          <el-option label="认证失败" :value="3" />
          <el-option label="已更换" :value="4" />
        </el-select>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="记录ID" width="90" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="user_phone" label="用户手机号" width="130">
          <template #default="{ row }">{{ row.user_phone || '-' }}</template>
        </el-table-column>
        <el-table-column prop="name" label="姓名" width="110">
          <template #default="{ row }">{{ row.name ? maskName(row.name) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="id_card" label="身份证号" width="200">
          <template #default="{ row }">{{ row.id_card ? maskIdCard(row.id_card) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="result_message" label="结果信息" min-width="160">
          <template #default="{ row }">{{ row.result_message || '-' }}</template>
        </el-table-column>
        <el-table-column prop="verified_at" label="认证通过时间" width="180">
          <template #default="{ row }">{{ row.verified_at ? formatDateTime(row.verified_at) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
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
        @size-change="onFilterChange"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { User } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref([])

const filters = reactive({
  status: null as number | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadRecords()
})

const onFilterChange = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getKycRecords({
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filters.status ?? -1
    })
    records.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载个人实名记录失败')
  } finally {
    loading.value = false
  }
}

const getStatusType = (status: number) => {
  const map: Record<number, string> = { 2: 'success', 1: 'warning', 0: 'info', 3: 'danger', 4: 'info' }
  return map[status] || 'info'
}

const getStatusText = (status: number) => {
  const map: Record<number, string> = { 0: '待认证', 1: '认证中', 2: '认证成功', 3: '认证失败', 4: '已更换' }
  return map[status] || '未知'
}

const maskName = (name: string) => {
  if (!name || name.length === 0) return ''
  if (name.length === 1) return name
  if (name.length === 2) return name[0] + '*'
  return name[0] + '*'.repeat(name.length - 2) + name[name.length - 1]
}

const maskIdCard = (id: string) => {
  if (!id || id.length < 8) return id
  return id.slice(0, 4) + '*'.repeat(id.length - 8) + id.slice(-4)
}
</script>

<style scoped>
.admin-kyc {
  min-height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
}
</style>
