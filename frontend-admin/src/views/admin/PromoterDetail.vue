<template>
  <div class="admin-promoter-detail page-fill">
    <div class="card">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Share /></el-icon>
          推广商详情
        </h3>
        <el-button @click="router.back()">返回</el-button>
      </div>

      <el-descriptions :column="3" border v-if="promoter">
        <el-descriptions-item label="推广码">
          <span class="mono">{{ promoter.aff_code || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="类型">
          <el-tag :type="promoter.referrer_type === 'user' ? 'primary' : 'warning'">
            {{ promoter.referrer_type === 'user' ? '用户推广' : '员工销售' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="名称">{{ promoter.name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="手机号">{{ promoter.phone || '-' }}</el-descriptions-item>
        <el-descriptions-item label="下级数">{{ promoter.sub_count }}</el-descriptions-item>
        <el-descriptions-item label="累计提成">
          <span :class="{ negative: promoter.commission_total < 0 }">¥{{ promoter.commission_total }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="上级" :span="3">{{ referrerText }}</el-descriptions-item>
      </el-descriptions>
    </div>

    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><User /></el-icon>
          下级用户
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
            @change="handleSearch"
          />
        </div>
      </div>

      <el-table :data="subUsers" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="90" />
        <el-table-column prop="username" label="用户名" min-width="140">
          <template #default="{ row }">{{ row.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" width="140">
          <template #default="{ row }">{{ row.phone || '-' }}</template>
        </el-table-column>
        <el-table-column label="实名状态" width="120">
          <template #default="{ row }">
            <el-tag :type="realnameTagType(row.realname_status)">{{ realnameText(row.realname_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '正常' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="注册时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadDetail"
        @size-change="handleSearch"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Share, User } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()

const referrerType = computed(() => String(route.params.type || ''))
const referrerId = computed(() => Number(route.params.id || 0))

const loading = ref(false)
const promoter = ref<any>(null)
const referrer = ref<any>(null)
const subUsers = ref<any[]>([])
const dateRange = ref<[string, string] | null>(null)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 上级归属：员工销售为平台自营体系；用户型推广无上级时为平台直营
const referrerText = computed(() => {
  if (!promoter.value) return ''
  if (promoter.value.referrer_type === 'staff') return '平台直营'
  return referrer.value?.name || '平台直营'
})

const realnameText = (status: number) => {
  const map: Record<number, string> = { 1: '个人实名', 2: '企业实名' }
  return map[status] || '未实名'
}

const realnameTagType = (status: number) => {
  const map: Record<number, string> = { 0: 'info', 1: 'success', 2: 'warning' }
  return map[status] || 'info'
}

const handleSearch = () => {
  pagination.page = 1
  loadDetail()
}

const loadDetail = async () => {
  if (!referrerType.value || !referrerId.value) return
  loading.value = true
  try {
    const res: any = await adminAPI.getPromoterDetail(referrerType.value, referrerId.value, {
      page: pagination.page,
      page_size: pagination.pageSize,
      start_date: dateRange.value?.[0],
      end_date: dateRange.value?.[1]
    })
    promoter.value = res.promoter || null
    referrer.value = res.referrer || null
    subUsers.value = res.sub_users?.list || []
    pagination.total = res.sub_users?.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadDetail()
})

// 路由参数变化（直接切换不同推广商）时重新加载
watch([referrerType, referrerId], () => {
  pagination.page = 1
  dateRange.value = null
  promoter.value = null
  referrer.value = null
  subUsers.value = []
  pagination.total = 0
  loadDetail()
})
</script>

<style scoped>
.admin-promoter-detail {
  min-height: 100%;
  gap: 16px;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.negative {
  color: var(--color-danger);
}
</style>