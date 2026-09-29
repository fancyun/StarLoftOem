<template>
  <div class="admin-finance">
    <div class="card">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Coin /></el-icon>
          财务统计
        </h3>
        <div class="filter-bar" style="margin: 0">
          <el-date-picker
            v-model="dateRange"
            type="daterange"
            value-format="YYYY-MM-DD"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            :clearable="false"
            @change="loadData"
          />
        </div>
      </div>

      <div class="stats-cards">
        <div class="stat-card">
          <div class="stat-label">总充值金额</div>
          <div class="stat-value">¥{{ recharge.total_amount || 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">充值订单数</div>
          <div class="stat-value">{{ recharge.total_orders || 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">支付宝充值</div>
          <div class="stat-value">¥{{ recharge.alipay_amount || 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">总消费金额</div>
          <div class="stat-value">¥{{ consume.total_amount || 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">消费笔数</div>
          <div class="stat-value">{{ consume.total_count || 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">实名消费</div>
          <div class="stat-value">
            ¥{{ consume.kyc_consume_amount || 0 }}
            <span class="stat-sub">({{ consume.kyc_consume_count || 0 }}笔)</span>
          </div>
        </div>
      </div>

      <div class="card" style="margin-top: var(--gap-lg)">
        <h3 class="section-title">
          <el-icon><TrendCharts /></el-icon>
          每日充值与消费
        </h3>
        <div ref="chartRef" style="height: 320px"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { Coin, TrendCharts } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'

const defaultStart = () => {
  const d = new Date()
  d.setDate(d.getDate() - 29)
  return d.toISOString().slice(0, 10)
}
const defaultEnd = () => new Date().toISOString().slice(0, 10)

const dateRange = ref<[string, string]>([defaultStart(), defaultEnd()])

const recharge = ref<any>({})
const consume = ref<any>({})
const chartRef = ref()
let chartInstance: ReturnType<typeof echarts.init> | null = null

const var_color_primary = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim() || '#006EFF'
)
const var_color_success = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-success').trim() || '#00A870'
)

onMounted(async () => {
  await loadData()
})

onBeforeUnmount(() => {
  chartInstance?.dispose()
})

const loadData = async () => {
  const params = {
    start_date: dateRange.value?.[0],
    end_date: dateRange.value?.[1]
  }
  try {
    const [summary, daily] = await Promise.all([
      adminAPI.getFinanceSummary(params) as Promise<any>,
      adminAPI.getFinanceDaily(params) as Promise<any>
    ])
    recharge.value = summary.recharge || {}
    consume.value = summary.consume || {}
    renderChart(daily.daily_stats || [])
  } catch (error: any) {
    ElMessage.error('加载财务数据失败')
  }
}

const renderChart = (daily: any[]) => {
  if (!chartRef.value) return
  const dates = daily.map((d) => d.date)
  const rechargeAmounts = daily.map((d) => d.recharge_amount)
  const consumeAmounts = daily.map((d) => d.consume_amount)

  chartInstance = echarts.init(chartRef.value)
  chartInstance.setOption({
    color: [var_color_primary.value, var_color_success.value],
    tooltip: { trigger: 'axis' },
    legend: { data: ['充值', '消费'] },
    grid: { left: 60, right: 20, top: 40, bottom: 30 },
    xAxis: { type: 'category', data: dates, axisLine: { lineStyle: { color: '#E5E6EB' } } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#F2F3F5' } } },
    series: [
      {
        name: '充值',
        type: 'line',
        smooth: true,
        symbolSize: 5,
        lineStyle: { width: 3 },
        data: rechargeAmounts
      },
      {
        name: '消费',
        type: 'line',
        smooth: true,
        symbolSize: 5,
        lineStyle: { width: 3 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(0, 168, 112, 0.25)' },
              { offset: 1, color: 'rgba(0, 168, 112, 0.02)' }
            ]
          }
        },
        data: consumeAmounts
      }
    ]
  })
}
</script>

<style scoped>
.admin-finance {
  min-height: 100%;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--gap-md);
}

.stat-card {
  background: var(--bg-soft);
  border-radius: var(--radius-md);
  padding: 16px 20px;
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
}

.stat-sub {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 400;
}
</style>