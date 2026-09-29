<template>
  <div class="admin-sms-templates page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          短信模板审核
        </h3>
        <div class="head-right">
          <el-select v-model="statusFilter" style="width: 150px" @change="onFilterChange">
            <el-option label="全部" :value="-1" />
            <el-option label="待平台审核" :value="3" />
            <el-option label="待上游审核" :value="0" />
            <el-option label="已通过" :value="1" />
            <el-option label="已驳回" :value="2" />
          </el-select>
          <el-button type="primary" :icon="Refresh" circle @click="loadRecords" />
        </div>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="系统ID" width="90" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="template_id" label="上游ID" width="110">
          <template #default="{ row }">{{ row.template_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="template_name" label="模板名称" min-width="140" />
        <el-table-column label="类型" width="80">
          <template #default="{ row }">{{ typeText(row.template_type) }}</template>
        </el-table-column>
        <el-table-column prop="sign_name" label="关联签名" min-width="120" />
        <el-table-column prop="template_content" label="模板内容" min-width="220" show-overflow-tooltip />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.status === 3" type="warning">待平台审核</el-tag>
            <el-tag v-else-if="row.status === 1" type="success">已通过</el-tag>
            <el-tag v-else-if="row.status === 2" type="danger">已驳回</el-tag>
            <el-tag v-else type="info">待上游审核</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="说明" min-width="120">
          <template #default="{ row }">{{ row.reason || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="提交时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="canWrite && row.status !== 3"
              link
              type="primary"
              :loading="queryingId === row.id"
              @click="handleQueryStatus(row)"
            >
              查询结果
            </el-button>
            <template v-if="canWrite && (row.status === 0 || row.status === 3)">
              <el-button link type="success" @click="handleApprove(row)">通过</el-button>
              <el-button link type="danger" @click="handleReject(row)">驳回</el-button>
            </template>
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
        @size-change="loadRecords"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document, Refresh } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 sms.templates.write 时隐藏查询结果/通过/驳回按钮
const canWrite = computed(() => adminStore.has('sms.templates.write'))

const loading = ref(false)
const queryingId = ref<number | null>(null)
const records = ref<any[]>([])
const statusFilter = ref(-1)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const typeText = (type: string) => {
  if (type === 'verify') return '验证码'
  if (type === 'marketing') return '营销'
  return '通知'
}

const onFilterChange = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.listSmsTemplates({
      status: statusFilter.value,
      page: pagination.page,
      page_size: pagination.pageSize
    })
    records.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载模板列表失败')
  } finally {
    loading.value = false
  }
}

const handleApprove = (row: any) => {
  // 待平台审核（3）：平台通过后才报备上游，随后转「待上游审核」
  const tip = row.status === 3
    ? `确认通过模板「${row.template_name}」？通过后将报备上游，之后需等待上游审核结果。`
    : `确认通过模板「${row.template_name}」？`
  ElMessageBox.confirm(tip, '通过模板', {
    confirmButtonText: '通过',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async () => {
      try {
        await adminAPI.reviewSmsTemplate(row.id, { status: 1 })
        ElMessage.success(row.status === 3 ? '已通过，已报备上游' : '已通过')
      } catch (error: any) {
        ElMessage.error(error?.message || '通过失败')
      }
      loadRecords()
    })
    .catch(() => {})
}

const handleReject = (row: any) => {
  ElMessageBox.prompt(`请输入驳回模板「${row.template_name}」的原因`, '驳回模板', {
    confirmButtonText: '驳回',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '驳回原因不能为空'
  })
    .then(async ({ value }) => {
      try {
        await adminAPI.reviewSmsTemplate(row.id, { status: 2, reason: value })
        ElMessage.success('已驳回')
      } catch (error: any) {
        ElMessage.error(error?.message || '驳回失败')
      }
      loadRecords()
    })
    .catch(() => {})
}

// 查询审核结果（调上游核对并回写本地，与用户端同款）
const handleQueryStatus = async (row: any) => {
  queryingId.value = row.id
  try {
    const res: any = await adminAPI.querySmsTemplate(row.id)
    if (res?.status === 1) {
      ElMessage.success('该模板已审核通过')
    } else if (res?.status === 2) {
      ElMessage.error(res?.reason || '该模板审核未通过')
    } else if (!res?.template_id) {
      ElMessage.info('该模板尚未报备上游，无审核结果')
    } else {
      ElMessage.info(`该模板上游${res?.reason || '审核中'}，请耐心等待`)
    }
    await loadRecords()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败，请稍后重试')
  } finally {
    queryingId.value = null
  }
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.admin-sms-templates {
  min-height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.muted {
  color: var(--text-muted);
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>