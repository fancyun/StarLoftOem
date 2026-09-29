<template>
  <div class="dashboard-container">
    <div class="content" v-loading="pageLoading">
      <!-- 欢迎横幅 -->
      <div class="welcome-banner">
        <div class="welcome-text">
          <h2 class="welcome-title">你好，{{ userInfo.username || maskPhone(userInfo.phone) || '用户' }}</h2>
          <p class="welcome-desc">欢迎使用{{ siteName }}控制台，管理你的云产品与账户资源。</p>
        </div>
        <div class="welcome-actions">
          <el-button v-if="!isKycVerified" @click="$router.push('/certification')">认证</el-button>
          <el-button type="primary" @click="$router.push('/balance')">充值</el-button>
        </div>
      </div>

      <!-- 账户概览 -->
      <div class="status-cards">
        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><Wallet /></el-icon>
            <span>账户余额</span>
          </div>
          <div class="card-body">
            <div class="amount">¥{{ userInfo.balance ?? '--' }}</div>
            <el-button size="small" @click="$router.push('/balance')">充值</el-button>
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><UserFilled /></el-icon>
            <span>实名状态</span>
          </div>
          <div class="card-body">
            <el-tag :type="kycTagType" size="large">{{ kycStatusText }}</el-tag>
            <el-button v-if="!isKycVerified" size="small" @click="$router.push('/certification')">去认证</el-button>
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <el-icon class="icon"><Key /></el-icon>
            <span>API 密钥</span>
          </div>
          <div class="card-body">
            <div class="api-key">{{ maskAPIKey(userInfo.api_key) }}</div>
            <el-button v-if="userInfo.api_key" size="small" @click="copyAPIKey">复制</el-button>
            <el-button size="small" @click="$router.push('/api')">管理</el-button>
          </div>
        </div>
      </div>

      <!-- 产品服务 -->
      <div class="card">
        <h3 class="section-title">
          <el-icon><Box /></el-icon>
          产品服务
        </h3>
        <div class="product-grid">
          <div class="product-item">
            <div class="product-icon fv">
              <el-icon><Avatar /></el-icon>
            </div>
            <div class="product-info">
              <div class="product-name">人脸核验</div>
              <div class="product-desc">活体人脸 · 有源/无源比对</div>
              <div class="product-stats">近 30 天调用 <b>{{ fvCallCount }}</b> 次</div>
            </div>
            <el-button type="primary" plain size="small" @click="$router.push('/fv')">管理</el-button>
          </div>

          <div class="product-item">
            <div class="product-icon sms">
              <el-icon><Message /></el-icon>
            </div>
            <div class="product-info">
              <div class="product-name">短信服务</div>
              <div class="product-desc">验证码 · 通知 · 签名短信</div>
              <div class="product-stats">累计发送 <b>{{ smsTotalCount }}</b> 条</div>
            </div>
            <el-button type="primary" plain size="small" @click="$router.push('/sms')">管理</el-button>
          </div>
        </div>
      </div>

      <!-- 使用趋势 -->
      <div class="card">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><TrendCharts /></el-icon>
            使用趋势（近 30 天）
          </h3>
        </div>
        <div ref="chartRef" class="trend-chart"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Wallet, UserFilled, Key, Box, Avatar, Message, TrendCharts } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { userAPI, smsAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { siteName } from '@/utils/promotion'

const userStore = useUserStore()

const userInfo = ref<any>({})
const pageLoading = ref(true)
const fvStats = ref<{ dates: string[]; counts: number[] }>({ dates: [], counts: [] })
const smsStats = ref<any>({})
const chartRef = ref()

let chartInstance: ReturnType<typeof echarts.init> | null = null
let disposed = false

const isKycVerified = computed(() => (userInfo.value.realname_status ?? 0) >= 1)

const kycStatusText = computed(() => {
  const v = userInfo.value.realname_status ?? 0
  if (v === 2) return '企业实名'
  if (v === 1) return '个人实名'
  return '未实名'
})

const kycTagType = computed(() => {
  const v = userInfo.value.realname_status ?? 0
  if (v >= 1) return 'success'
  return 'warning'
})

const fvCallCount = computed(() => fvStats.value.counts.reduce((sum, n) => sum + (n || 0), 0))

const smsTotalCount = computed(() => smsStats.value.total_count ?? 0)

const var_color_primary = computed(() =>
  getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim() || '#1CD5C7'
)

// ===== 工具函数 =====
const maskPhone = (phone: string) => {
  if (!phone) return ''
  return phone.substring(0, 3) + '****' + phone.substring(7)
}

const maskAPIKey = (key: string) => {
  if (!key) return '未设置'
  return key.substring(0, 8) + '...' + key.substring(key.length - 4)
}

const copyAPIKey = () => {
  navigator.clipboard.writeText(userInfo.value.api_key)
  ElMessage.success('API Key 已复制到剪贴板')
}

const loadData = async () => {
  try {
    const profile = await userAPI.getProfile()
    userInfo.value = profile
    userStore.setUserInfo(profile)
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
  }
}

const loadTrend = async () => {
  try {
    const [calls, sms] = await Promise.all([
      userAPI.getCallStats(),
      smsAPI.getSMSStats({ days: 30 })
    ])
    fvStats.value = calls
    smsStats.value = sms
    renderChart()
  } catch (error) {
    console.error('加载使用趋势失败:', error)
  }
}

// 当前图表所处的断点（用于跨断点时重算坐标轴留白与图例尺寸）
let chartNarrow: boolean | null = null

// 按最新数据与当前视口宽度应用图表配置
const applyChartOption = () => {
  if (!chartInstance) return

  const calls = fvStats.value
  const smsDaily: any[] = smsStats.value.daily || []
  const smsMap: Record<string, number> = {}
  smsDaily.forEach((d) => {
    smsMap[d.date] = d.count
  })

  // 窄屏压缩坐标轴留白，避免绘图区被挤扁
  const narrow = window.innerWidth <= 768
  chartNarrow = narrow

  chartInstance.setOption({
    color: [var_color_primary.value, '#2E7FFF'],
    tooltip: { trigger: 'axis', confine: true },
    legend: {
      data: ['人脸核验', '短信发送'],
      bottom: 0,
      itemWidth: narrow ? 14 : 25,
      itemHeight: narrow ? 8 : 14,
      textStyle: { fontSize: narrow ? 11 : 12 }
    },
    xAxis: {
      type: 'category',
      data: calls.dates,
      axisLine: { lineStyle: { color: '#E5E6EB' } },
      axisLabel: { fontSize: narrow ? 10 : 12 }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#F2F3F5' } },
      axisLabel: { fontSize: narrow ? 10 : 12 }
    },
    grid: { left: narrow ? 32 : 40, right: narrow ? 12 : 20, top: 20, bottom: narrow ? 32 : 40 },
    series: [
      {
        name: '人脸核验',
        data: calls.counts,
        type: 'line',
        smooth: true,
        symbolSize: 5,
        lineStyle: { width: 2.5 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(28, 213, 199, 0.22)' },
              { offset: 1, color: 'rgba(28, 213, 199, 0.02)' }
            ]
          }
        }
      },
      {
        name: '短信发送',
        data: calls.dates.map((d) => smsMap[d] || 0),
        type: 'line',
        smooth: true,
        symbolSize: 5,
        lineStyle: { width: 2.5 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(46, 127, 255, 0.2)' },
              { offset: 1, color: 'rgba(46, 127, 255, 0.02)' }
            ]
          }
        }
      }
    ]
  })
}

const renderChart = () => {
  if (disposed || !chartRef.value) return
  chartInstance = echarts.init(chartRef.value)
  applyChartOption()
}

// 视口尺寸变化时重绘图表（手机横竖屏切换、PC 缩放窗口、侧栏展开等场景）
const handleChartResize = () => {
  if (disposed || !chartInstance) return
  chartInstance.resize()
  // 跨断点时重算坐标轴与图例尺寸，避免残留过大留白
  if ((window.innerWidth <= 768) !== chartNarrow) {
    applyChartOption()
  }
}

onMounted(() => {
  loadData()
  loadTrend()
  window.addEventListener('resize', handleChartResize)
})

onBeforeUnmount(() => {
  disposed = true
  window.removeEventListener('resize', handleChartResize)
  chartInstance?.dispose()
  chartInstance = null
})
</script>

<style scoped>
.dashboard-container {
  min-height: 100%;
}

.content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* ===== 欢迎横幅 ===== */
.welcome-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px 28px;
  background: linear-gradient(135deg, var(--color-primary) 0%, #15C5BA 100%);
  border-radius: var(--radius-lg);
}

.welcome-title {
  color: #fff;
  font-size: 22px;
  font-weight: 600;
}

.welcome-desc {
  margin-top: 6px;
  color: rgba(255, 255, 255, 0.9);
  font-size: 14px;
}

.welcome-actions :deep(.el-button) {
  margin-left: 12px;
}

/* ===== 账户概览卡片 ===== */
.status-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 20px;
}

.card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  color: var(--text-muted);
  font-size: 14px;
}

.card-header .icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: var(--color-primary-light);
  color: var(--color-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.amount {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
}

.api-key {
  font-family: 'Courier New', monospace;
  color: var(--text-secondary);
  font-size: 14px;
}

/* ===== 区块标题 ===== */
.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  margin-bottom: 16px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

/* ===== 产品服务 ===== */
.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 20px;
}

/* ===== 使用趋势图容器（高度随断点变化，配合窗口 resize 重绘） ===== */
.trend-chart {
  width: 100%;
  height: 300px;
}

.product-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-card);
  transition: box-shadow 0.2s;
}

.product-item:hover {
  box-shadow: var(--shadow-md);
}

.product-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  flex-shrink: 0;
}

.product-icon.fv {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.product-icon.sms {
  background: var(--color-info-light);
  color: var(--color-info);
}

.product-info {
  flex: 1;
  min-width: 0;
}

.product-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.product-desc {
  margin-top: 2px;
  font-size: 13px;
  color: var(--text-muted);
}

.product-stats {
  margin-top: 6px;
  font-size: 13px;
  color: var(--text-secondary);
}

.product-stats b {
  color: var(--text-primary);
}

/* ========== 响应式：平板 / 手机 ========== */

/* 平板（≤1024px）：收紧横幅内边距 */
@media (max-width: 1024px) {
  .welcome-banner {
    padding: 20px 22px;
  }
}

/* 手机（≤768px）：横幅竖排、网格紧凑、图表降高 */
@media (max-width: 768px) {
  .content {
    gap: 16px;
  }

  .welcome-banner {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
    padding: 20px;
  }

  .welcome-title {
    font-size: 18px;
  }

  .welcome-desc {
    font-size: 13px;
  }

  .welcome-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  /* 重置 Element 相邻按钮的默认左间距，统一由 flex gap 控制 */
  .welcome-actions :deep(.el-button),
  .welcome-actions :deep(.el-button + .el-button) {
    margin-left: 0;
  }

  .status-cards,
  .product-grid {
    gap: 12px;
  }

  .card {
    padding: 16px;
  }

  .amount {
    font-size: 22px;
  }

  .product-item {
    gap: 12px;
    padding: 16px;
  }

  .product-icon {
    width: 42px;
    height: 42px;
    font-size: 20px;
  }

  .trend-chart {
    height: 220px;
  }
}

/* 小屏手机（≤480px）：强制单列铺满，操作按钮等宽便于点按 */
@media (max-width: 480px) {
  .status-cards,
  .product-grid {
    grid-template-columns: 1fr;
  }

  .welcome-actions {
    width: 100%;
  }

  .welcome-actions :deep(.el-button) {
    flex: 1;
  }
}
</style>
