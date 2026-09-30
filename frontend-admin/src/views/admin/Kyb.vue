<template>
  <div class="admin-kyb page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><OfficeBuilding /></el-icon>
          企业实名管理
        </h3>
        <div class="head-actions">
          <el-select v-model="filterStatus" style="width: 150px" @change="handleFilterChange">
            <el-option label="全部状态" :value="-1" />
            <el-option label="待人工审核" :value="4" />
            <el-option label="已通过" :value="2" />
            <el-option label="未通过" :value="3" />
            <el-option label="待法人扫脸" :value="1" />
            <el-option label="待四要素" :value="0" />
          </el-select>
          <el-button v-if="canWrite" type="primary" @click="openVerifyDialog">开通企业实名</el-button>
        </div>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="记录ID" width="90" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="company_name" label="企业名称" width="220">
          <template #default="{ row }">{{ row.company_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="credit_code" label="统一社会信用代码" width="200">
          <template #default="{ row }">{{ row.credit_code || '-' }}</template>
        </el-table-column>
        <el-table-column prop="legal_name" label="法人姓名" width="100">
          <template #default="{ row }">{{ row.legal_name ? maskName(row.legal_name) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="verified_at" label="开通时间" width="180">
          <template #default="{ row }">{{ row.verified_at ? formatDateTime(row.verified_at) : formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="result_message" label="结果说明" min-width="180">
          <template #default="{ row }">{{ row.result_message || '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="canWrite && row.status === 4"
              type="primary"
              link
              @click="openReviewDialog(row)"
            >
              审核
            </el-button>
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
        @current-change="loadRecords"
        @size-change="loadRecords"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>

    <!-- 开通企业实名对话框 -->
    <el-dialog v-model="verifyVisible" title="开通企业实名" width="520px" :close-on-click-modal="false">
      <el-form :model="verifyForm" :rules="verifyRules" ref="verifyFormRef" label-width="130px">
        <el-form-item label="用户ID" prop="user_id">
          <el-input-number v-model="verifyForm.user_id" :min="1" :controls="false" style="width: 100%" placeholder="请输入用户ID" />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            填写用户管理列表中的用户ID
          </div>
        </el-form-item>
        <el-form-item label="企业名称" prop="company_name">
          <el-input v-model="verifyForm.company_name" placeholder="请输入企业名称" maxlength="100" />
        </el-form-item>
        <el-form-item label="信用代码" prop="credit_code">
          <el-input v-model="verifyForm.credit_code" placeholder="请输入统一社会信用代码" maxlength="50" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="verifyVisible = false">取消</el-button>
        <el-button type="primary" :loading="verifyLoading" @click="handleVerify">确定开通</el-button>
      </template>
    </el-dialog>

    <!-- 人工审核对话框（未配置工商四要素时，企业实名由用户提交、后台审核） -->
    <el-dialog v-model="reviewVisible" title="企业实名人工审核" width="520px" :close-on-click-modal="false">
      <el-descriptions :column="1" border size="small" style="margin-bottom: 16px">
        <el-descriptions-item label="用户ID">{{ reviewRow.user_id }}</el-descriptions-item>
        <el-descriptions-item label="企业名称">{{ reviewRow.company_name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="统一社会信用代码">{{ reviewRow.credit_code || '-' }}</el-descriptions-item>
        <el-descriptions-item label="法人姓名">{{ reviewRow.legal_name ? maskName(reviewRow.legal_name) : '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-form label-width="90px">
        <el-form-item label="驳回原因">
          <el-input v-model="reviewReason" type="textarea" :rows="3" maxlength="200" placeholder="驳回时填写，将展示给用户；通过时可留空" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="danger" :loading="reviewLoading" @click="handleReview(false)">驳回</el-button>
        <el-button type="primary" :loading="reviewLoading" @click="handleReview(true)">通过并开通</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { OfficeBuilding } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 sys.users.write 时隐藏开通企业实名按钮
const canWrite = computed(() => adminStore.has('sys.users.write'))
const loading = ref(false)
const records = ref([])
// 状态筛选（-1=全部）
const filterStatus = ref(-1)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const verifyVisible = ref(false)
const verifyLoading = ref(false)
const verifyFormRef = ref<FormInstance>()

const verifyForm = reactive({
  user_id: undefined as number | undefined,
  company_name: '',
  credit_code: ''
})

const verifyRules: FormRules = {
  user_id: [{ required: true, message: '请输入用户ID', trigger: 'blur' }],
  company_name: [{ required: true, message: '请输入企业名称', trigger: 'blur' }],
  credit_code: [{ required: true, message: '请输入统一社会信用代码', trigger: 'blur' }]
}

// 人工审核状态
const reviewVisible = ref(false)
const reviewLoading = ref(false)
const reviewReason = ref('')
const reviewRow = reactive({
  id: 0,
  user_id: 0,
  company_name: '',
  credit_code: '',
  legal_name: ''
})

onMounted(() => {
  loadRecords()
})

const loadRecords = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getKybRecords({
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filterStatus.value
    })
    records.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载企业实名记录失败')
  } finally {
    loading.value = false
  }
}

const handleFilterChange = () => {
  pagination.page = 1
  loadRecords()
}

const openReviewDialog = (row: any) => {
  reviewRow.id = row.id
  reviewRow.user_id = row.user_id
  reviewRow.company_name = row.company_name || ''
  reviewRow.credit_code = row.credit_code || ''
  reviewRow.legal_name = row.legal_name || ''
  reviewReason.value = ''
  reviewVisible.value = true
}

const handleReview = async (approve: boolean) => {
  if (!approve && !reviewReason.value.trim()) {
    ElMessage.warning('请填写驳回原因')
    return
  }
  reviewLoading.value = true
  try {
    await adminAPI.reviewKybManual({
      id: reviewRow.id,
      action: approve ? 'approve' : 'reject',
      reason: reviewReason.value.trim()
    })
    ElMessage.success(approve ? '企业实名已开通' : '已驳回')
    reviewVisible.value = false
    loadRecords()
  } catch (error: any) {
    // 错误消息已由请求拦截器统一提示
  } finally {
    reviewLoading.value = false
  }
}

const openVerifyDialog = () => {
  verifyForm.user_id = undefined
  verifyForm.company_name = ''
  verifyForm.credit_code = ''
  verifyFormRef.value?.clearValidate()
  verifyVisible.value = true
}

const handleVerify = async () => {
  if (!verifyFormRef.value) return
  await verifyFormRef.value.validate(async (valid) => {
    if (!valid) return
    verifyLoading.value = true
    try {
      await adminAPI.verifyKyb({
        user_id: verifyForm.user_id!,
        company_name: verifyForm.company_name.trim(),
        credit_code: verifyForm.credit_code.trim()
      })
      ElMessage.success('企业实名已开通')
      verifyVisible.value = false
      loadRecords()
    } catch (error: any) {
      // 错误消息已由请求拦截器统一提示
    } finally {
      verifyLoading.value = false
    }
  })
}

const getStatusType = (status: number) => {
  const map: Record<number, string> = { 2: 'success', 1: 'warning', 0: 'info', 3: 'danger', 4: 'warning' }
  return map[status] || 'info'
}

const getStatusText = (status: number) => {
  const map: Record<number, string> = { 2: '已通过', 1: '审核中', 0: '待审核', 3: '未通过', 4: '待人工审核' }
  return map[status] || '未知'
}

const maskName = (name: string) => {
  if (!name || name.length === 0) return ''
  if (name.length === 1) return name
  if (name.length === 2) return name[0] + '*'
  return name[0] + '*'.repeat(name.length - 2) + name[name.length - 1]
}
</script>

<style scoped>
.admin-kyb {
  min-height: 100%;
}
.head-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>