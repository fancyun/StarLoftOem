<template>
  <div class="admin-sms-stats">
    <div class="stats-cards">
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon count">
            <el-icon><ChatDotSquare /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatCount(stats.totalCount) }}</div>
            <div class="stat-label">成功发送条数</div>
          </div>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon amount">
            <el-icon><Coin /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">¥{{ formatAmount(stats.totalAmount) }}</div>
            <div class="stat-label">总消费金额</div>
          </div>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon today">
            <el-icon><Clock /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ formatCount(todayCount) }}</div>
            <div class="stat-label">今日成功条数</div>
          </div>
        </div>
      </el-card>
    </div>

    <div class="card">
      <div class="chart-head">
        <h3 class="section-title">
          <el-icon><TrendCharts /></el-icon>
          发送趋势
        </h3>
        <el-radio-group v-model="days" @change="loadChart">
          <el-radio-button :value="7">近7天</el-radio-button>
          <el-radio-button :value="30">近30天</el-radio-button>
        </el-radio-group>
      </div>
      <div ref="chartRef" style="height: 300px"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { ChatDotSquare, Coin, Clock, TrendCharts } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'

const var_color_primary = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim() || '#006EFF'
)
const var_color_success = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-success').trim() || '#00A870'
)

const days = ref(7)
const stats = ref({
  totalCount: 0,
  totalAmount: 0,
  daily: [] as { date: string; count: number; amount: number }[]
})

const chartRef = ref()
let chartInstance: ReturnType<typeof echarts.init> | null = null
let disposed = false

const todayCount = computed(() => {
  const daily = stats.value.daily
  return daily.length ? daily[daily.length - 1].count : 0
})

const formatCount = (n: number) => (n || 0).toLocaleString('zh-CN')
const formatAmount = (n: number) => (n || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

onMounted(async () => {
  await loadStats()
  await loadChart()
})

onBeforeUnmount(() => {
  disposed = true
  chartInstance?.dispose()
})

const loadStats = async () => {
  try {
    const res: any = await adminAPI.getSmsStats({ days: days.value })
    stats.value = {
      totalCount: res.total_count || 0,
      totalAmount: res.total_amount || 0,
      daily: res.daily || []
    }
    renderChart()
  } catch (error: any) {
    ElMessage.error('加载短信统计数据失败')
  }
}

const loadChart = async () => {
  await loadStats()
}

const renderChart = () => {
  if (disposed) return
  if (!chartRef.value) return

  const daily = stats.value.daily
  chartInstance = echarts.init(chartRef.value)
  chartInstance.setOption({
    color: [var_color_primary.value, var_color_success.value],
    legend: { data: ['成功发送条数', '消费金额'], top: 0, right: 0 },
    xAxis: { type: 'category', data: daily.map((d) => d.date), axisLine: { lineStyle: { color: '#E5E6EB' } } },
    yAxis: [
      { type: 'value', name: '条数', minInterval: 1, splitLine: { lineStyle: { color: '#F2F3F5' } } },
      { type: 'value', name: '金额(元)', splitLine: { show: false }, axisLine: { show: false }, axisTick: { show: false } }
    ],
    grid: { left: 50, right: 60, top: 40, bottom: 30 },
    series: [
      {
        name: '成功发送条数',
        data: daily.map((d) => d.count),
        type: 'line',
        smooth: true,
        symbolSize: 6,
        lineStyle: { width: 3 }
      },
      {
        name: '消费金额',
        data: daily.map((d) => Number((d.amount || 0).toFixed(2))),
        type: 'line',
        yAxisIndex: 1,
        smooth: true,
        symbolSize: 6,
        lineStyle: { width: 3 }
      }
    ],
    tooltip: {
      trigger: 'axis',
      formatter: (params: any) => {
        const idx = params[0]?.dataIndex
        const item = daily[idx]
        if (!item) return ''
        return `${item.date}<br/>成功发送条数：${formatCount(item.count)}<br/>消费金额：¥${formatAmount(item.amount)}`
      }
    }
  })
}
</script>

<style scoped>
.admin-sms-stats {
  min-height: 100%;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: var(--gap-lg);
  margin-bottom: var(--gap-lg);
}

.stat-card {
  transition: transform 0.15s, box-shadow 0.15s;
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
  flex-shrink: 0;
}

.stat-icon.count { color: var(--color-primary); background: var(--color-primary-light); }
.stat-icon.amount { color: var(--color-success); background: var(--color-success-light); }
.stat-icon.today { color: #FF9D00; background: #FFF7E8; }

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

.chart-head {
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
</style>
