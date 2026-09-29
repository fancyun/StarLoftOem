<template>
  <div class="sms-records-container page-fill">
    <div class="content">
      <div class="card card-fill">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Document /></el-icon>
            发送记录
          </h3>
          <div class="filters">
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              value-format="YYYY-MM-DD"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              size="default"
            />
            <el-button @click="handleSearch">查询</el-button>
          </div>
        </div>

        <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
          <el-table-column prop="biz_no" label="流水号" width="180" />
          <el-table-column prop="phone_number_set" label="手机号" min-width="160" show-overflow-tooltip />
          <el-table-column prop="sign_name" label="签名" min-width="120" />
          <el-table-column prop="template_id" label="模板ID" width="110" />
          <el-table-column prop="phone_count" label="条数" width="80" />
          <el-table-column label="扣费" width="130">
            <template #default="{ row }">{{ chargeText(row) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag v-if="row.status === 0" type="success">成功</el-tag>
              <el-tag v-else type="danger">失败</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="fail_message" label="失败原因" min-width="140" show-overflow-tooltip />
          <el-table-column prop="created_at" label="发送时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>

        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="loadRecords"
          @size-change="handleSearch"
          style="margin-top: 20px; justify-content: center"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { smsAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dateRange = ref<[string, string] | null>(null)

// 扣费展示：失败未计费，资源包支付显示扣减条数（金额为 0），余额支付显示实扣金额
const chargeText = (row: any) => {
  if (row.status === 1) return '-'
  if (row.pay_type === 0) return `资源包 ${row.pack_count || 0} 条`
  return `¥${row.amount}`
}

const loadRecords = async () => {
  loading.value = true
  try {
    const params: any = {
      page: page.value,
      page_size: pageSize.value
    }
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_date = dateRange.value[0]
      params.end_date = dateRange.value[1]
    }
    const res = await smsAPI.listSendRecords(params)
    records.value = res.list
    total.value = res.total
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  page.value = 1
  loadRecords()
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.sms-records-container {
  min-height: 0;
}

.content {
  display: flex;
  flex-direction: column;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.filters {
  display: flex;
  gap: 12px;
  align-items: center;
}
</style>