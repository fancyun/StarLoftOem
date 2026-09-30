<template>
  <div class="admin-notify-records page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Bell /></el-icon>
        通知重试
      </h3>

      <div class="filter-bar">
        <el-select v-model="filters.status" placeholder="推送状态" clearable style="width: 140px" @change="handleSearch">
          <el-option label="待推" :value="0" />
          <el-option label="已放弃" :value="2" />
        </el-select>
        <el-select v-model="filters.bizType" placeholder="业务类型" clearable style="width: 160px" @change="handleSearch">
          <el-option label="认证结果" value="fv_result" />
          <el-option label="短信回执" value="sms_receipt" />
          <el-option label="短信回复" value="sms_reply" />
          <el-option label="短信审核状态" value="sms_status" />
        </el-select>
        <el-input
          v-model="filters.keyword"
          placeholder="业务单号/关联记录ID/目标地址"
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
        <el-table-column prop="biz_no" label="业务单号" width="220">
          <template #default="{ row }">
            <span class="break-all mono">{{ row.biz_no }}</span>
          </template>
        </el-table-column>
        <el-table-column label="业务类型" width="120">
          <template #default="{ row }">
            <el-tag>{{ bizTypeText(row.biz_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="record_id" label="关联记录ID" width="110" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="target_url" label="目标地址" min-width="220" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="fail_times" label="失败次数" width="90" />
        <el-table-column label="下次重试" width="180">
          <template #default="{ row }">{{ row.next_retry_at ? formatDateTime(row.next_retry_at) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="last_error" label="最后错误" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">{{ row.last_error || '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="150">
          <template #default="{ row }">
            <el-button size="small" @click="viewPayload(row)">报文</el-button>
            <el-button
              v-if="canWrite && row.status !== 1"
              size="small"
              type="warning"
              :loading="retryingKey === rowKey(row)"
              @click="handleRetry(row)"
            >
              立即重推
            </el-button>
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

    <!-- 推送报文 -->
    <el-dialog v-model="payloadVisible" title="推送报文" width="720px" top="6vh">
      <el-descriptions :column="1" border style="margin-bottom: 16px" v-if="payloadRow">
        <el-descriptions-item label="业务类型">{{ bizTypeText(payloadRow.biz_type) }}</el-descriptions-item>
        <el-descriptions-item label="目标地址">
          <span class="break-all">{{ payloadRow.target_url }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <pre class="payload-pre">{{ payloadText }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Bell } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 ops.notify.write 时隐藏「立即重推」按钮
const canWrite = computed(() => adminStore.has('ops.notify.write'))

const loading = ref(false)
const records = ref<any[]>([])
const retryingKey = ref<string | null>(null)
const payloadVisible = ref(false)
const payloadRow = ref<any>(null)

const filters = reactive({
  status: null as number | null,
  bizType: '',
  keyword: '',
  dateRange: null as [string, string] | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const payloadText = computed(() => {
  const raw = payloadRow.value?.payload
  if (!raw) return '（空）'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return String(raw)
  }
})

const bizTypeText = (type: string) => {
  const map: Record<string, string> = { fv_result: '认证结果', sms_receipt: '短信回执', sms_reply: '短信回复', sms_status: '短信审核状态' }
  return map[type] || type || '-'
}

const statusText = (status: number) => {
  const map: Record<number, string> = { 0: '待推', 1: '成功', 2: '已放弃' }
  return map[status] || '未知'
}

const statusType = (status: number) => {
  const map: Record<number, string> = { 0: 'warning', 1: 'success', 2: 'danger' }
  return map[status] || 'info'
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
    const res: any = await adminAPI.getNotifyRecords({
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filters.status ?? undefined,
      biz_type: filters.bizType || undefined,
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

const viewPayload = (row: any) => {
  payloadRow.value = row
  payloadVisible.value = true
}

// 立即重推：已放弃记录需二次确认（强制重推会重置失败次数，下游可能重复回调）
const handleRetry = async (row: any) => {
  const force = row.status === 2
  if (force) {
    try {
      await ElMessageBox.confirm(
        '该通知已放弃重推，强制重推会重置失败次数并可能造成下游重复回调，确认继续？',
        '强制重推',
        { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      return
    }
  }
  retryingKey.value = rowKey(row)
  try {
    await adminAPI.retryNotifyRecord({ biz_type: row.biz_type, biz_no: row.biz_no, force })
    ElMessage.success('已触发重推')
    await loadRecords()
  } catch {
    // 错误消息已由请求拦截器统一提示（业务 code=400 时为后端 message）
  } finally {
    retryingKey.value = null
  }
}

// rowKey 通知记录以「业务类型 + 业务单号」为唯一键
const rowKey = (row: any) => `${row.biz_type}|${row.biz_no}`
</script>

<style scoped>
.admin-notify-records {
  min-height: 100%;
}

.break-all {
  word-break: break-all;
}

.payload-pre {
  margin: 0;
  padding: 12px 16px;
  max-height: 420px;
  overflow: auto;
  background: var(--bg-soft);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>