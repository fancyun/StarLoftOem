<template>
  <div class="records-container page-fill">
    <div class="content">
      <div class="card card-fill">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Document /></el-icon>
            认证记录
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
            <el-button @click="loadRecords">查询</el-button>
          </div>
        </div>

        <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
          <el-table-column prop="biz_no" label="流水号" width="180" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.status === 2" type="success">认证成功</el-tag>
              <el-tag v-else-if="row.status === 3" type="danger">认证失败</el-tag>
              <el-tag v-else-if="row.status === 4" type="info">已取消</el-tag>
              <el-tag v-else-if="row.status === 5" type="info">{{ row.is_refunded ? '超时已退款' : '超时未计费' }}</el-tag>
              <el-tag v-else-if="row.status === 6" type="info">发起失败（未扣费）</el-tag>
              <el-tag v-else-if="row.status === 1" type="warning">认证中</el-tag>
              <el-tag v-else type="info">待认证</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="result_message" label="结果" width="120" />
          <el-table-column prop="result_code" label="结果码" width="120">
            <template #default="{ row }">{{ row.result_code || '-' }}</template>
          </el-table-column>
          <el-table-column label="资源包扣减" width="100">
            <template #default="{ row }">{{ row.pack_count ? row.pack_count + ' 次' : '-' }}</template>
          </el-table-column>
          <el-table-column label="费用" width="90">
            <template #default="{ row }">{{ row.pay_type === 2 ? '资源包' : (row.cost ? '¥' + row.cost : '-') }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button link type="primary" @click="viewDetail(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="loadRecords"
          @size-change="loadRecords"
          style="margin-top: 20px; justify-content: center"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { userAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dateRange = ref([])

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
    
    const res = await userAPI.getFvRecords(params)
    records.value = res.list
    total.value = res.total
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const viewDetail = (_row: any) => {
  ElMessage.info('详情功能待实现')
}

onMounted(() => {
  loadRecords()
})
</script>

<style scoped>
.records-container {
  min-height: 0;
}

.content {
  display: flex;
  flex-direction: column;
}

.filters {
  display: flex;
  gap: 12px;
  align-items: center;
}
</style>
