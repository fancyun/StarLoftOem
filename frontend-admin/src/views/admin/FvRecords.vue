<template>
  <div class="admin-fv-records page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Document /></el-icon>
        认证记录
      </h3>

      <div class="filter-bar">
        <el-select v-model="authFilters.status" placeholder="认证状态" clearable style="width: 150px">
          <el-option label="待认证" :value="0" />
          <el-option label="认证中" :value="1" />
          <el-option label="已完成" :value="2" />
          <el-option label="失败" :value="3" />
          <el-option label="已取消" :value="4" />
          <el-option label="超时已退款" :value="5" />
          <el-option label="发起失败（未扣费）" :value="6" />
        </el-select>
        <el-button type="primary" icon="Search" @click="loadAuthOrders">搜索</el-button>
      </div>

      <el-table :data="authOrders" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="biz_no" label="业务流水号" width="200" />
        <el-table-column prop="user_phone" label="用户手机号" width="140" />
        <el-table-column prop="status" label="状态" width="160">
          <template #default="{ row }">
            <el-tag :type="getAuthStatusType(row.status)">
              {{ row.status === 5 && !row.is_refunded ? '超时未计费' : getAuthStatusText(row.status) }}
            </el-tag>
            <div v-if="(row.status === 3 || row.status === 6) && (row.result_message || row.result_code)" class="fail-reason">
              {{ row.result_message || `结果码 ${row.result_code}` }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="result_code" label="结果码" width="120">
          <template #default="{ row }">{{ row.result_code || '-' }}</template>
        </el-table-column>
        <el-table-column label="是否退款" width="100">
          <template #default="{ row }">{{ row.is_refunded ? '是' : '否' }}</template>
        </el-table-column>
        <el-table-column label="资源包扣减" width="100">
          <template #default="{ row }">{{ row.pack_count ? row.pack_count + ' 次' : '-' }}</template>
        </el-table-column>
        <el-table-column label="金额" width="100">
          <template #default="{ row }">{{ row.pay_type === 2 ? '资源包' : (row.cost ? '¥' + row.cost : '-') }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button size="small" @click="viewAuthDetail(row)">详情</el-button>
            <el-button v-if="canWrite" size="small" :loading="queryingId === row.id" @click="queryAuthResult(row)">查询结果</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="authPagination.page"
        v-model:page-size="authPagination.pageSize"
        :total="authPagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadAuthOrders"
        @size-change="loadAuthOrders"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>

    <!-- 认证记录详情 -->
    <el-dialog v-model="authDetailVisible" title="认证记录详情" width="720px" top="6vh">
      <el-descriptions :column="2" border v-if="authDetail">
        <el-descriptions-item label="业务流水号" :span="2">{{ authDetail.biz_no || '-' }}</el-descriptions-item>
        <el-descriptions-item label="记录ID">{{ authDetail.id }}</el-descriptions-item>
        <el-descriptions-item label="用户ID">{{ authDetail.user_id }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="getAuthStatusType(authDetail.status)">{{ getAuthStatusText(authDetail.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="消耗金额">¥{{ authDetail.cost }}</el-descriptions-item>
        <el-descriptions-item label="结果码">{{ authDetail.result_code || '-' }}</el-descriptions-item>
        <el-descriptions-item label="结果信息">
          {{ authDetail.result_message || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="回调次数">{{ authDetail.notify_times }}</el-descriptions-item>
        <el-descriptions-item label="回调状态">{{ getNotifyStatusText(authDetail.notify_status) }}</el-descriptions-item>
        <el-descriptions-item label="是否退款">{{ authDetail.is_refunded ? '是' : '否' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatDateTime(authDetail.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="完成时间">{{ authDetail.finished_at ? formatDateTime(authDetail.finished_at) : '-' }}</el-descriptions-item>
        <el-descriptions-item label="返回地址" :span="2" v-if="authDetail.return_url">
          <span class="break-all">{{ authDetail.return_url }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="通知地址" :span="2" v-if="authDetail.notify_url">
          <span class="break-all">{{ authDetail.notify_url }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="上游凭证" :span="2" v-if="authDetail.up_token || authDetail.up_biz_id">
          token={{ authDetail.up_token || '-' }} /
          biz_id={{ authDetail.up_biz_id || '-' }} /
          request_id={{ authDetail.up_request_id || '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()
// 写权限：无 fv.records.write 时隐藏「查询结果」按钮（该操作会写库）
const canWrite = computed(() => adminStore.has('fv.records.write'))

const loading = ref(false)
const authOrders = ref([])

const authFilters = reactive({
  status: null as number | null
})

const authPagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const authDetailVisible = ref(false)
const authDetail = ref<any>(null)
// 正在查询结果的记录 ID（按钮 loading）
const queryingId = ref<number | null>(null)

onMounted(() => {
  loadAuthOrders()
})

const loadAuthOrders = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getFvRecords({
      page: authPagination.page,
      page_size: authPagination.pageSize,
      status: authFilters.status ?? undefined
    })
    authOrders.value = response.records || response.list || []
    authPagination.total = response.total || 0
  } catch (error: any) {
    ElMessage.error('加载认证记录失败')
  } finally {
    loading.value = false
  }
}

const getAuthStatusType = (status: number) => {
  const map: Record<number, string> = { 0: 'info', 1: 'warning', 2: 'success', 3: 'danger', 4: 'info', 5: 'info', 6: 'info' }
  return map[status] || 'info'
}

const getAuthStatusText = (status: number) => {
  const map: Record<number, string> = { 0: '待认证', 1: '认证中', 2: '已完成', 3: '失败', 4: '已取消', 5: '超时已退款', 6: '发起失败（未扣费）' }
  return map[status] || '未知'
}

const getNotifyStatusText = (status: number) => {
  const map: Record<number, string> = { 0: '待通知', 1: '通知成功', 2: '通知失败' }
  return map[status] ?? '未知'
}

const viewAuthDetail = async (row: any) => {
  authDetailVisible.value = true
  authDetail.value = row
  try {
    const detail: any = await adminAPI.getRecordDetail(row.id)
    authDetail.value = detail
  } catch (error: any) {
    // 详情请求失败时保留列表行数据展示
  }
}

// 查询结果（调上游核对并按上游结果回写本地，与后台签名/模板同款）
const queryAuthResult = async (row: any) => {
  queryingId.value = row.id
  try {
    const res: any = await adminAPI.queryFvRecordResult(row.id)
    if (res?.status === 2) {
      ElMessage.success(res?.query_message || '该记录认证成功')
    } else if (res?.status === 3) {
      ElMessage.error(res?.query_message || '该记录认证失败')
    } else {
      ElMessage.info(res?.query_message || '查询完成')
    }
    await loadAuthOrders()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败，请稍后重试')
  } finally {
    queryingId.value = null
  }
}
</script>

<style scoped>
.admin-fv-records {
  min-height: 100%;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.fail-reason {
  font-size: 12px;
  color: var(--color-danger);
  margin-top: 4px;
  line-height: 1.4;
}

.break-all {
  word-break: break-all;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
}
</style>
