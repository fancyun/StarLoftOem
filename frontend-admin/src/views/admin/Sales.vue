<template>
  <div class="admin-sales page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><TrendCharts /></el-icon>
          销售业绩
        </h3>
        <div class="head-right">
          <el-date-picker
            v-model="selectedMonth"
            type="month"
            value-format="YYYY-MM"
            format="YYYY-MM"
            placeholder="选择月份"
            :clearable="false"
            style="width: 150px"
            @change="handleReload"
          />
          <el-select
            v-if="adminStore.isSuper"
            v-model="selectedStaffId"
            placeholder="查看员工"
            style="width: 200px"
            clearable
            @change="handleReload"
          >
            <el-option
              v-for="item in staffOptions"
              :key="item.id"
              :label="item.nickname || item.username"
              :value="item.id"
            />
          </el-select>
        </div>
      </div>

      <div class="stats-grid" v-loading="summaryLoading">
        <div class="stat-item">
          <div class="stat-label">本月提成（{{ summary.month || selectedMonth }}）</div>
          <div class="stat-value">¥{{ summary.month_commission ?? 0 }}</div>
        </div>
        <div class="stat-item">
          <div class="stat-label">累计提成</div>
          <div class="stat-value">¥{{ summary.total_commission ?? 0 }}</div>
        </div>
        <div class="stat-item">
          <div class="stat-label">本月新增推广客户</div>
          <div class="stat-value">{{ summary.month_sub_user_count ?? 0 }}</div>
          <div class="stat-sub">累计 {{ summary.sub_user_count ?? 0 }}</div>
        </div>
        <div class="stat-item">
          <div class="stat-label">提成比例（人脸核验 / 短信）</div>
          <div class="stat-value">{{ rateText(summary.rate_fv) }} / {{ rateText(summary.rate_sms) }}</div>
        </div>
      </div>

      <div class="link-row">
        <span class="link-label">推广码</span>
        <el-input :value="summary.aff_code || '-'" readonly style="width: 200px" />
        <span class="link-label">推广链接</span>
        <el-input :value="summary.promo_link || '-'" readonly class="link-input" />
        <el-button :disabled="!summary.promo_link" @click="copyLink">复制链接</el-button>
      </div>
      <div class="link-tip">
        推广链接按「控制台注册地址 + 推广码」生成，客户经该链接注册后消费即计入本账号提成（提成 = 利润 × 比例）。员工销售提成长期有效（不设归因期限），仅记录流水，结算由平台统一处理。下方明细与「本月」指标均按所选月份（{{ selectedMonth }}）统计；提成流水按发生时间、推广客户按注册时间归月。
      </div>

      <el-tabs v-model="activeTab" style="margin-top: 8px">
        <el-tab-pane label="提成流水" name="commissions">
          <el-table :data="commissions" style="width: 100%" v-loading="tableLoading">
            <el-table-column prop="id" label="流水 ID" width="90" />
            <el-table-column prop="user_id" label="来源用户 ID" width="110" />
            <el-table-column label="金额" width="120">
              <template #default="{ row }">
                <span :style="{ color: row.amount >= 0 ? 'var(--color-success)' : 'var(--color-danger)' }">
                  {{ row.amount >= 0 ? '+' : '' }}¥{{ row.amount }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="业务类型" width="130">
              <template #default="{ row }">{{ bizTypeLabel(row.biz_type) }}</template>
            </el-table-column>
            <el-table-column prop="ref_type" label="关联表" width="180">
              <template #default="{ row }">{{ row.ref_type || '-' }}</template>
            </el-table-column>
            <el-table-column prop="ref_id" label="关联 ID" width="100" />
            <el-table-column prop="remark" label="备注" min-width="180">
              <template #default="{ row }">{{ row.remark || '-' }}</template>
            </el-table-column>
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-model:current-page="commissionPager.page"
            v-model:page-size="commissionPager.pageSize"
            :total="commissionPager.total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @current-change="loadCommissions"
            @size-change="loadCommissions"
            style="margin-top: 16px; justify-content: flex-end"
          />
        </el-tab-pane>

        <el-tab-pane label="推广客户" name="users">
          <el-table :data="subUsers" style="width: 100%" v-loading="tableLoading">
            <el-table-column prop="id" label="用户 ID" width="100" />
            <el-table-column prop="username" label="用户名" width="180" />
            <el-table-column label="手机号" width="150">
              <template #default="{ row }">{{ maskPhone(row.phone) }}</template>
            </el-table-column>
            <el-table-column label="实名状态" width="120">
              <template #default="{ row }">{{ realnameLabel(row.realname_status) }}</template>
            </el-table-column>
            <el-table-column label="账号状态" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '正常' : '禁用' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="注册时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <el-pagination
            v-model:current-page="userPager.page"
            v-model:page-size="userPager.pageSize"
            :total="userPager.total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @current-change="loadSubUsers"
            @size-change="loadSubUsers"
            style="margin-top: 16px; justify-content: flex-end"
          />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { TrendCharts } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const adminStore = useAdminStore()

const summaryLoading = ref(false)
const tableLoading = ref(false)
const activeTab = ref('commissions')
const summary = reactive<any>({})
const commissions = ref<any[]>([])
const subUsers = ref<any[]>([])
const staffOptions = ref<{ id: number; username: string; nickname: string }[]>([])
const selectedStaffId = ref<number | undefined>(undefined)

// 销售报告按月查看：默认当前月，格式 YYYY-MM
const currentMonth = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}
const selectedMonth = ref(currentMonth())

const commissionPager = reactive({ page: 1, pageSize: 20, total: 0 })
const userPager = reactive({ page: 1, pageSize: 20, total: 0 })

// 提成比例展示：0 表示不提成
const rateText = (rate?: number) => (typeof rate === 'number' ? `${(rate * 100).toFixed(2)}%` : '-')

const BIZ_TYPE_LABELS: Record<string, string> = {
  pack_purchase: '资源包购买',
  sms_send: '短信发送',
  fv_auth: '人脸核验',
  refund: '退款冲回'
}
const bizTypeLabel = (bizType: string) => BIZ_TYPE_LABELS[bizType] || bizType || '-'

const realnameLabel = (status: number) => {
  const map: Record<number, string> = { 0: '未实名', 1: '个人实名', 2: '企业实名' }
  return map[status] || '-'
}

// 手机号脱敏（后台展示同样避免明文）
const maskPhone = (phone?: string) => {
  if (!phone || phone.length < 7) return phone || '-'
  return phone.slice(0, 3) + '****' + phone.slice(-4)
}

const loadSummary = async () => {
  summaryLoading.value = true
  try {
    const res: any = await adminAPI.getSalesSummary(selectedStaffId.value, selectedMonth.value)
    Object.assign(summary, res || {})
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    summaryLoading.value = false
  }
}

const loadCommissions = async () => {
  tableLoading.value = true
  try {
    const res: any = await adminAPI.getSalesCommissions({
      staff_id: selectedStaffId.value,
      month: selectedMonth.value,
      page: commissionPager.page,
      page_size: commissionPager.pageSize
    })
    commissions.value = res.list || []
    commissionPager.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    tableLoading.value = false
  }
}

const loadSubUsers = async () => {
  tableLoading.value = true
  try {
    const res: any = await adminAPI.getSalesUsers({
      staff_id: selectedStaffId.value,
      month: selectedMonth.value,
      page: userPager.page,
      page_size: userPager.pageSize
    })
    subUsers.value = res.list || []
    userPager.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    tableLoading.value = false
  }
}

// 切换月份或查看对象后重置分页并重新加载
const handleReload = () => {
  commissionPager.page = 1
  userPager.page = 1
  loadSummary()
  loadCommissions()
  loadSubUsers()
}

const copyLink = async () => {
  if (!summary.promo_link) return
  try {
    await navigator.clipboard.writeText(summary.promo_link)
    ElMessage.success('推广链接已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

// 超管可切换查看任意员工，需先加载员工列表
const loadStaffOptions = async () => {
  if (!adminStore.isSuper) return
  try {
    const res: any = await adminAPI.listAdmins({ page: 1, page_size: 200 })
    staffOptions.value = res.list || []
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}

onMounted(() => {
  loadStaffOptions()
  loadSummary()
  loadCommissions()
  loadSubUsers()
})
</script>

<style scoped>
.admin-sales {
  min-height: 100%;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat-item {
  padding: 14px 16px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-page);
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.stat-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
}

.stat-sub {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.link-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.link-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.link-input {
  flex: 1;
  min-width: 260px;
}

.link-tip {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-muted);
}
</style>