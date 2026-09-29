<template>
  <div class="admin-sms-records page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          发送记录
        </h3>
        <div class="filter-bar">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            style="width: 260px"
          />
          <el-button type="primary" @click="loadRecords">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </div>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="biz_no" label="业务流水号" width="180" />
        <el-table-column prop="user_phone" label="用户手机号" width="130" />
        <el-table-column prop="template_id" label="模板ID" width="110" />
        <el-table-column prop="sign_name" label="签名" min-width="120" show-overflow-tooltip />
        <el-table-column label="条数" width="80">
          <template #default="{ row }">{{ row.phone_count }}</template>
        </el-table-column>
        <el-table-column label="扣费" width="130">
          <template #default="{ row }">{{ chargeText(row) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="160">
          <template #default="{ row }">
            <el-tag :type="row.status === 0 ? 'success' : 'danger'">
              {{ row.status === 0 ? '成功' : '失败' }}
            </el-tag>
            <div v-if="row.status === 1 && row.fail_message" class="fail-reason">
              {{ row.fail_message }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="发送时间" width="180">
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
        @size-change="loadRecords"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Document } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])
const dateRange = ref<[string, string] | null>(null)

// 扣费展示：失败未计费，资源包支付显示扣减条数（金额为 0），余额支付显示实扣金额
const chargeText = (row: any) => {
  if (row.status === 1) return '-'
  if (row.pay_type === 0) return `资源包 ${row.pack_count || 0} 条`
  return `¥${row.amount}`
}

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadRecords()
})

const loadRecords = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getSmsRecords({
      page: pagination.page,
      page_size: pagination.pageSize,
      start_date: dateRange.value?.[0] || undefined,
      end_date: dateRange.value?.[1] || undefined
    })
    records.value = res.list || []
    pagination.total = res.total || 0
  } catch (error: any) {
    ElMessage.error('加载发送记录失败')
  } finally {
    loading.value = false
  }
}

const handleReset = () => {
  dateRange.value = null
  pagination.page = 1
  loadRecords()
}
</script>

<style scoped>
.admin-sms-records {
  min-height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 0;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.fail-reason {
  font-size: 12px;
  color: var(--color-danger);
  margin-top: 4px;
  line-height: 1.4;
}
</style>
