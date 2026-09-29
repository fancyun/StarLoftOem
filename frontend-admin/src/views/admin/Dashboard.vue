<template>
  <div class="admin-dashboard page-fill">
    <div class="stats-cards">
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon users">
            <el-icon><User /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.totalUsers }}</div>
            <div class="stat-label">总用户数</div>
          </div>
        </div>
      </el-card>
      
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon orders">
            <el-icon><Document /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ stats.todayOrders }}</div>
            <div class="stat-label">今日订单数</div>
          </div>
        </div>
      </el-card>
      
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon revenue">
            <el-icon><Coin /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">¥{{ stats.todayRevenue }}</div>
            <div class="stat-label">今日收入</div>
          </div>
        </div>
      </el-card>
      
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon monthly">
            <el-icon><TrendCharts /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">¥{{ stats.monthRevenue }}</div>
            <div class="stat-label">本月收入</div>
          </div>
        </div>
      </el-card>
    </div>

    <el-row :gutter="20" class="chart-row">
      <el-col :span="24">
        <div class="card">
          <h3 class="section-title">
            <el-icon><Coin /></el-icon>
            收入趋势
          </h3>
          <div ref="revenueChartRef" style="height: 300px"></div>
        </div>
      </el-col>
    </el-row>

    <div class="card card-fill recent-orders">
      <h3 class="section-title">
        <el-icon><Document /></el-icon>
        最近订单
      </h3>
      <el-table :data="recentOrders" class="table-fill" height="100%" style="width: 100%">
        <el-table-column prop="biz_no" label="业务流水号" width="200" />
        <el-table-column prop="user_phone" label="用户手机号" width="140" />
        <el-table-column prop="name" label="姓名" width="100">
          <template #default="{ row }">{{ maskName(row.name) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cost" label="金额" width="100">
          <template #default="{ row }">¥{{ row.cost }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

// 在 TS 里读取 CSS 变量，保证图表色和主题同步
const var_color_success = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-success').trim() || '#00A870'
)

const stats = ref({
  totalUsers: 0,
  todayOrders: 0,
  todayRevenue: 0,
  monthRevenue: 0
})

const recentOrders = ref([])
const revenueChartRef = ref()

let revenueChartInstance: ReturnType<typeof echarts.init> | null = null
let disposed = false

onMounted(async () => {
  await loadStats()
  await loadCharts()
  await loadRecentOrders()
})

onBeforeUnmount(() => {
  disposed = true
  revenueChartInstance?.dispose()
})

const loadStats = async () => {
  try {
    const response = await adminAPI.getStatsOverview()
    stats.value = {
      totalUsers: response.total_users || 0,
      todayOrders: response.today_orders || 0,
      // 平台收入按支付记录口径统计（含人工支付），不取认证订单收入
      todayRevenue: response.today_payment_amount || 0,
      monthRevenue: response.month_payment_amount || 0
    }
  } catch (error: any) {
    console.error('加载统计数据失败:', error)
    ElMessage.error('加载统计数据失败')
  }
}

const loadCharts = async () => {
  try {
    const revenueRes = await adminAPI.getStatsRevenue(7, 'payment')

    // 组件已卸载，则跳过渲染，避免操作已销毁的 DOM
    if (disposed) return
    if (!revenueChartRef.value) return

    // 收入趋势图（支付记录口径）
    revenueChartInstance = echarts.init(revenueChartRef.value)
    revenueChartInstance.setOption({
      color: [var_color_success.value],
      xAxis: { type: 'category', data: revenueRes.dates, axisLine: { lineStyle: { color: '#E5E6EB' } } },
      yAxis: { type: 'value', splitLine: { lineStyle: { color: '#F2F3F5' } } },
      grid: { left: 40, right: 20, top: 20, bottom: 30 },
      series: [{
        data: revenueRes.amounts, type: 'line', smooth: true, symbolSize: 6,
        lineStyle: { width: 3 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(0, 168, 112, 0.25)' },
              { offset: 1, color: 'rgba(0, 168, 112, 0.02)' }
            ]
          }
        }
      }],
      tooltip: { trigger: 'axis' }
    })
  } catch (error: any) {
    if (disposed) return
    console.error('加载图表数据失败:', error)
    ElMessage.error('加载图表数据失败')
  }
}

const loadRecentOrders = async () => {
  try {
    const response: any = await adminAPI.getRecentRecords(10)
    recentOrders.value = response.list || response || []
  } catch (error: any) {
    console.error('加载最近订单失败:', error)
    ElMessage.error('加载最近订单失败')
  }
}

const maskName = (name: string) => {
  if (!name || name.length === 0) return ''
  if (name.length === 1) return name
  if (name.length === 2) return name[0] + '*'
  return name[0] + '*'.repeat(name.length - 2) + name[name.length - 1]
}

const getStatusType = (status: number) => {
  const map: Record<number, string> = {
    0: 'info',
    1: 'warning',
    2: 'success',
    3: 'danger',
    4: 'info',
    5: 'info',
    6: 'info'
  }
  return map[status] || 'info'
}

const getStatusText = (status: number) => {
  const map: Record<number, string> = {
    0: '待认证',
    1: '认证中',
    2: '已完成',
    3: '失败',
    4: '已取消',
    5: '超时已退款',
    6: '发起失败（未扣费）'
  }
  return map[status] || '未知'
}
</script>

<style scoped>
.admin-dashboard {
  min-height: 100%;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: var(--gap-lg);
  margin-bottom: var(--gap-lg);
}

.stat-card {
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
}
.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md) !important;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: var(--color-primary);
  background: var(--color-primary-light);
  flex-shrink: 0;
}

.stat-icon.users   { color: #006EFF; background: #E8F3FF; }
.stat-icon.orders  { color: #FF9D00; background: #FFF7E8; }
.stat-icon.revenue { color: var(--color-success); background: var(--color-success-light); }
.stat-icon.monthly { color: #F54A45; background: #FFECE8; }

.stat-info {
  flex: 1;
  min-width: 0;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 4px;
}

.chart-row {
  margin-bottom: var(--gap-lg);
}

.recent-orders {
  margin-top: var(--gap-lg);
}
</style>
