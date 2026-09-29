<template>
  <div class="admin-sms-signs page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><EditPen /></el-icon>
          短信签名审核
        </h3>
        <div class="head-right">
          <el-select v-model="statusFilter" style="width: 140px" @change="onFilterChange">
            <el-option label="全部" :value="-1" />
            <el-option label="待审核" :value="0" />
            <el-option label="已通过" :value="2" />
            <el-option label="已驳回" :value="3" />
          </el-select>
          <el-button type="primary" :icon="Refresh" circle @click="loadRecords" />
        </div>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="系统ID" width="90" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="up_sign_id" label="上游ID" width="110">
          <template #default="{ row }">{{ row.up_sign_id || '-' }}</template>
        </el-table-column>
        <el-table-column prop="sign_name" label="签名内容" min-width="140" />
        <el-table-column label="提交用户手机" min-width="130">
          <template #default="{ row }">{{ row.user_phone || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag v-if="row.status === 2" type="success">已通过</el-tag>
            <el-tag v-else-if="row.status === 3" type="danger">已驳回</el-tag>
            <el-tag v-else-if="row.status === 1" type="warning">审核中</el-tag>
            <el-tag v-else type="info">待审核</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可见性" width="90">
          <template #default="{ row }">
            <el-tag v-if="row.is_public === 1" type="warning">公共</el-tag>
            <el-tag v-else type="info">私有</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="result_message" label="审核结果" min-width="170">
          <template #default="{ row }">
            <span>{{ row.result_message || '-' }}</span>
            <el-button v-if="canWrite" link type="primary" class="edit-result" @click="editResultMessage(row)">编辑</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="提交时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="330" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="viewAttach(row)">附件</el-button>
            <el-button
              v-if="canWrite"
              link
              type="primary"
              :loading="queryingId === row.id"
              @click="handleQueryStatus(row)"
            >
              查询结果
            </el-button>
            <el-button v-if="canWrite" link type="primary" @click="handleTogglePublic(row)">
              {{ row.is_public === 1 ? '设为私有' : '设为公共' }}
            </el-button>
            <template v-if="canWrite && row.status === 0">
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

    <!-- 签名资质附件 -->
    <el-dialog v-model="attachVisible" title="签名资质附件" width="760px" top="6vh">
      <template v-if="attachRow">
        <el-descriptions :column="1" border style="margin-bottom: 16px">
          <el-descriptions-item label="签名内容">{{ attachRow.sign_name }}</el-descriptions-item>
          <el-descriptions-item label="提交用户">ID {{ attachRow.user_id }}（{{ attachRow.user_phone || '-' }}）</el-descriptions-item>
        </el-descriptions>
        <div class="attach-grid">
          <div class="attach-item">
            <div class="attach-label">营业执照</div>
            <el-image
              v-if="attachRow.credit_code_url"
              :src="attachUrls['credit_code_url']"
              fit="contain"
              class="attach-img"
              :preview-src-list="attachUrls['credit_code_url'] ? [attachUrls['credit_code_url']] : []"
              preview-teleported
            >
              <template #placeholder><div class="attach-tip">加载中…</div></template>
              <template #error><div class="attach-tip">加载失败</div></template>
            </el-image>
            <span v-else class="muted">无</span>
          </div>
          <div class="attach-item">
            <div class="attach-label">身份证正面</div>
            <el-image
              v-if="attachRow.id_card_front"
              :src="attachUrls['id_card_front']"
              fit="contain"
              class="attach-img"
              :preview-src-list="attachUrls['id_card_front'] ? [attachUrls['id_card_front']] : []"
              preview-teleported
            >
              <template #placeholder><div class="attach-tip">加载中…</div></template>
              <template #error><div class="attach-tip">加载失败</div></template>
            </el-image>
            <span v-else class="muted">无</span>
          </div>
          <div class="attach-item">
            <div class="attach-label">身份证反面</div>
            <el-image
              v-if="attachRow.id_card_back"
              :src="attachUrls['id_card_back']"
              fit="contain"
              class="attach-img"
              :preview-src-list="attachUrls['id_card_back'] ? [attachUrls['id_card_back']] : []"
              preview-teleported
            >
              <template #placeholder><div class="attach-tip">加载中…</div></template>
              <template #error><div class="attach-tip">加载失败</div></template>
            </el-image>
            <span v-else class="muted">无</span>
          </div>
          <div class="attach-item">
            <div class="attach-label">意愿承诺函</div>
            <el-image
              v-if="attachRow.sx_commits"
              :src="attachUrls['sx_commits']"
              fit="contain"
              class="attach-img"
              :preview-src-list="attachUrls['sx_commits'] ? [attachUrls['sx_commits']] : []"
              preview-teleported
            >
              <template #placeholder><div class="attach-tip">加载中…</div></template>
              <template #error><div class="attach-tip">加载失败</div></template>
            </el-image>
            <span v-else class="muted">无</span>
          </div>
          <div class="attach-item">
            <div class="attach-label">签名授权书</div>
            <el-image
              v-if="attachRow.auth_letter"
              :src="attachUrls['auth_letter']"
              fit="contain"
              class="attach-img"
              :preview-src-list="attachUrls['auth_letter'] ? [attachUrls['auth_letter']] : []"
              preview-teleported
            >
              <template #placeholder><div class="attach-tip">加载中…</div></template>
              <template #error><div class="attach-tip">加载失败</div></template>
            </el-image>
            <span v-else class="muted">无</span>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { EditPen, Refresh } from '@element-plus/icons-vue'
import { adminAPI, loadAuthImage } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 sms.signs.write 时隐藏查询结果/通过/驳回/编辑按钮
const canWrite = computed(() => adminStore.has('sms.signs.write'))

const loading = ref(false)
const records = ref<any[]>([])
const statusFilter = ref(-1)
const queryingId = ref<number | null>(null)
const attachVisible = ref(false)
const attachRow = ref<any>(null)
const attachUrls = ref<Record<string, string>>({})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 附件图片 URL 受 UploadAuth 保护（需 admin JWT），浏览器 <img> 直连会 403；
// 打开弹窗时逐个携带 token 以 blob 拉取，转 object URL 后展示。
const attachKeys = ['credit_code_url', 'id_card_front', 'id_card_back', 'sx_commits', 'auth_letter']
const revokeAttachUrls = () => {
  Object.values(attachUrls.value).forEach((u) => {
    if (u) URL.revokeObjectURL(u)
  })
  attachUrls.value = {}
}
const viewAttach = async (row: any) => {
  attachRow.value = row
  attachVisible.value = true
  revokeAttachUrls()
  for (const key of attachKeys) {
    if (row[key]) {
      try {
        attachUrls.value[key] = await loadAuthImage(row[key])
      } catch (e) {
        attachUrls.value[key] = ''
      }
    }
  }
}
watch(attachVisible, (visible) => {
  if (!visible) revokeAttachUrls()
})

const onFilterChange = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.listSmsSigns({
      status: statusFilter.value,
      page: pagination.page,
      page_size: pagination.pageSize
    })
    records.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载签名列表失败')
  } finally {
    loading.value = false
  }
}

const handleApprove = (row: any) => {
  ElMessageBox.confirm(`确认通过签名「${row.sign_name}」？`, '通过签名', {
    confirmButtonText: '通过',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async () => {
      await adminAPI.reviewSmsSign(row.id, { status: 2 })
      ElMessage.success('已通过')
      loadRecords()
    })
    .catch(() => {})
}

const handleReject = (row: any) => {
  ElMessageBox.prompt(`请输入驳回签名「${row.sign_name}」的原因`, '驳回签名', {
    confirmButtonText: '驳回',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '驳回原因不能为空'
  })
    .then(async ({ value }) => {
      await adminAPI.reviewSmsSign(row.id, { status: 3, reason: value })
      ElMessage.success('已驳回')
      loadRecords()
    })
    .catch(() => {})
}

// 查询审核结果（调上游核对并回写本地，与用户端同款）
const handleQueryStatus = async (row: any) => {
  queryingId.value = row.id
  try {
    const res: any = await adminAPI.querySmsSign(row.id)
    if (res?.status === 2) {
      ElMessage.success('该签名已审核通过')
    } else if (res?.status === 3) {
      ElMessage.error(res?.result_message || '该签名审核未通过')
    } else if (!res?.up_sign_id) {
      ElMessage.info('该签名尚未报备上游，无审核结果')
    } else {
      ElMessage.info(`该签名上游${res?.result_message || '审核中'}，请耐心等待`)
    }
    await loadRecords()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败，请稍后重试')
  } finally {
    queryingId.value = null
  }
}

// 设为公共/私有：仅改可见性、不动上游签名；仅「已通过」的签名可设为公共（后端校验）
const handleTogglePublic = (row: any) => {
  const makePublic = row.is_public !== 1
  const tip = makePublic ? '设为公共后所有账号可见，并可用于绑定短信模板。' : ''
  ElMessageBox.confirm(`确认将签名「${row.sign_name}」${makePublic ? '设为公共' : '设为私有'}？${tip}`, makePublic ? '设为公共' : '设为私有', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
    .then(async () => {
      await adminAPI.setSmsSignPublic(row.id, makePublic ? 1 : 0)
      ElMessage.success('已保存')
      loadRecords()
    })
    .catch(() => {})
}

// 修改审核结果/失败原因文本（不改审核状态，留空即清空）
const editResultMessage = (row: any) => {
  ElMessageBox.prompt('修改审核结果 / 失败原因（留空表示清空）', `编辑「${row.sign_name}」`, {
    confirmButtonText: '保存',
    cancelButtonText: '取消',
    inputType: 'textarea',
    inputValue: row.result_message || '',
    inputValidator: (value: string) => (value || '').length <= 255 || '最多 255 个字符'
  })
    .then(async ({ value }) => {
      await adminAPI.updateSmsSignResultMessage(row.id, { result_message: value || '' })
      ElMessage.success('已保存')
      loadRecords()
    })
    .catch(() => {})
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.admin-sms-signs {
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

.edit-result {
  margin-left: 8px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.attach-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.attach-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.attach-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.attach-img {
  width: 100%;
  height: 200px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-soft);
  cursor: zoom-in;
}

.attach-tip {
  width: 100%;
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: var(--text-muted);
}
</style>