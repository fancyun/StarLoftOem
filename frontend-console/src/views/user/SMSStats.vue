<template>
  <div class="sms-stats-container">
    <div class="content" v-loading="loading">
      <!-- 统计卡片 -->
      <div class="status-cards">
        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><Message /></el-icon>
            <span>累计成功条数</span>
          </div>
          <div class="card-body">
            <div class="amount">{{ stats.total_count ?? 0 }}<span class="price-unit">条</span></div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><Wallet /></el-icon>
            <span>累计消费金额</span>
          </div>
          <div class="card-body">
            <div class="amount">¥{{ stats.total_amount ?? '0.00' }}</div>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><TrendCharts /></el-icon>
            <span>统计周期</span>
          </div>
          <div class="card-body">
            <div class="amount">{{ days }}<span class="price-unit">天</span></div>
          </div>
        </div>
      </div>

      <!-- 按日发送趋势 -->
      <div class="card">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><TrendCharts /></el-icon>
            发送趋势（近 {{ days }} 天）
          </h3>
          <el-radio-group v-model="days" size="small" @change="loadStats">
            <el-radio-button :value="7">7天</el-radio-button>
            <el-radio-button :value="30">30天</el-radio-button>
          </el-radio-group>
        </div>
        <div ref="chartRef" style="height: 300px"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import * as echarts from 'echarts'
import { smsAPI } from '@/api'

const loading = ref(false)
const stats = ref<any>({})
const days = ref(7)
const chartRef = ref()
let chartInstance: ReturnType<typeof echarts.init> | null = null
let disposed = false

const var_color_primary = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim() || '#006EFF'
)
const var_color_success = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-success').trim() || '#00A870'
)

const loadStats = async () => {
  loading.value = true
  try {
    const res = await smsAPI.getSMSStats({ days: days.value })
    stats.value = res
    renderChart(res.daily || [])
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const renderChart = (daily: any[]) => {
  if (disposed) return
  if (!chartRef.value) return
  if (!chartInstance) {
    chartInstance = echarts.init(chartRef.value)
  }
  chartInstance.setOption({
    color: [var_color_primary.value, var_color_success.value],
    legend: { data: ['成功发送条数', '消费金额'], top: 0, right: 0 },
    xAxis: {
      type: 'category',
      data: daily.map((d) => d.date),
      axisLine: { lineStyle: { color: '#E5E6EB' } }
    },
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
        lineStyle: { width: 3 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(37, 99, 235, 0.25)' },
              { offset: 1, color: 'rgba(37, 99, 235, 0.02)' }
            ]
          }
        }
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
        return `${item.date}<br/>成功发送条数：${item.count || 0}<br/>消费金额：¥${Number(item.amount || 0).toFixed(2)}`
      }
    }
  })
}

onMounted(() => {
  loadStats()
})

onBeforeUnmount(() => {
  disposed = true
  chartInstance?.dispose()
  chartInstance = null
})
</script>

<style scoped>
.sms-stats-container {
  min-height: 100%;
}

.content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.status-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  color: var(--text-muted);
  font-size: 14px;
}

.icon {
  font-size: 18px;
}

.card-body .amount {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
}

.price-unit {
  font-size: 14px;
  font-weight: 400;
  color: var(--text-muted);
  margin-left: 4px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
</style>