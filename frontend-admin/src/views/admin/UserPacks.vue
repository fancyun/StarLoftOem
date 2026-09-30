<template>
  <div class="admin-user-packs page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Box /></el-icon>
        用户资源包 · {{ productLabel }}
      </h3>

      <div class="filter-bar">
        <el-input
          v-model="filters.userId"
          placeholder="用户ID"
          style="width: 140px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />

        <el-select v-model="filters.status" placeholder="状态" clearable style="width: 140px" @change="handleSearch">
          <el-option label="有效" :value="1" />
          <el-option label="已耗尽" :value="0" />
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

        <el-button type="primary" icon="Search" @click="handleSearch">搜索</el-button>
      </div>

      <el-table :data="records" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="user_id" label="用户ID" width="90" />
        <el-table-column prop="phone" label="手机号" width="140">
          <template #default="{ row }">{{ row.phone || '-' }}</template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="120">
          <template #default="{ row }">{{ row.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="pack_name" label="资源包名称" min-width="160" show-overflow-tooltip />
        <el-table-column label="产品" width="130">
          <template #default="{ row }">
            <el-tag :type="productTagType(row)">{{ row.product || row.scope || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="total_count" label="总量" width="90" />
        <el-table-column prop="remaining_count" label="剩余" width="90" />
        <el-table-column label="快照价" width="100">
          <template #default="{ row }">¥{{ row.price }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '有效' : '已耗尽' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="购买时间" width="180">
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
        @size-change="handleSearch"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Box } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const route = useRoute()

// 当前一级产品（页面按一级分区拆分：/sms/user-packs → sms、/fv/user-packs → fv）
const product = computed<'sms' | 'fv'>(() => (route.path.startsWith('/sms') ? 'sms' : 'fv'))
const productLabel = computed(() => (product.value === 'fv' ? '人脸核验' : '短信服务'))

const loading = ref(false)
const records = ref<any[]>([])

const filters = reactive({
  userId: '',
  status: null as number | null,
  dateRange: null as [string, string] | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 产品标签：人脸核验系 primary，短信系 success
const productTagType = (row: any) => {
  const product: string = row.product || row.scope || ''
  return product.startsWith('fv') ? 'primary' : 'success'
}

onMounted(() => {
  loadRecords()
})

// 在两个一级产品页面间切换（同一组件实例复用）时重新加载
watch(product, () => {
  pagination.page = 1
  loadRecords()
})

const handleSearch = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getUserPacks({
      page: pagination.page,
      page_size: pagination.pageSize,
      scope: product.value,
      user_id: filters.userId || undefined,
      status: filters.status ?? undefined,
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
</script>

<style scoped>
.admin-user-packs {
  min-height: 100%;
}
</style>