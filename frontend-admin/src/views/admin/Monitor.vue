<template>
  <div class="admin-monitor page-fill">
    <div class="card-head">
      <h3 class="section-title">
        <el-icon><Monitor /></el-icon>
        系统监控
      </h3>
      <div class="head-right">
        <el-switch v-model="autoRefresh" active-text="30s 自动刷新" />
        <el-button type="primary" :icon="Refresh" :loading="loading" @click="loadMonitor">刷新</el-button>
      </div>
    </div>

    <div class="monitor-grid" v-loading="loading">
      <!-- 数据库 -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">数据库</span>
          <el-tag v-if="data" :type="data.database?.ok ? 'success' : 'danger'">
            {{ data.database?.ok ? '正常' : '异常' }}
          </el-tag>
        </div>
        <div v-if="data?.database?.error" class="monitor-error">{{ data.database.error }}</div>
        <ul class="kv-list">
          <li v-for="row in databaseRows" :key="row.label">
            <span class="kv-label">{{ row.label }}</span>
            <span class="kv-value">{{ row.value }}</span>
          </li>
        </ul>
      </div>

      <!-- Redis -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">Redis</span>
          <el-tag v-if="data" :type="data.redis?.ok ? 'success' : 'danger'">
            {{ data.redis?.ok ? '正常' : '异常' }}
          </el-tag>
        </div>
        <div v-if="data?.redis?.error" class="monitor-error">{{ data.redis.error }}</div>
        <ul class="kv-list">
          <li v-for="row in redisRows" :key="row.label">
            <span class="kv-label">{{ row.label }}</span>
            <span class="kv-value">{{ row.value }}</span>
          </li>
        </ul>
      </div>

      <!-- 进程 -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">进程</span>
        </div>
        <ul class="kv-list">
          <li v-for="row in processRows" :key="row.label">
            <span class="kv-label">{{ row.label }}</span>
            <span class="kv-value">{{ row.value }}</span>
          </li>
        </ul>
      </div>

      <!-- 运行配置 -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">运行配置</span>
        </div>
        <ul class="kv-list">
          <li v-for="row in configRows" :key="row.label">
            <span class="kv-label">{{ row.label }}</span>
            <span class="kv-value break-all">{{ row.value }}</span>
          </li>
        </ul>
      </div>

      <!-- 日志文件 -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">日志文件</span>
        </div>
        <el-table :data="logFiles" size="small" style="width: 100%">
          <el-table-column prop="name" label="文件" min-width="120" />
          <el-table-column label="大小" width="100">
            <template #default="{ row }">{{ row.exists ? humanSize(row.size_bytes) : '-' }}</template>
          </el-table-column>
          <el-table-column label="最后写入" min-width="160">
            <template #default="{ row }">{{ row.exists ? formatDateTime(row.modified_at) : '-' }}</template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 库表概览 -->
      <div class="card monitor-card">
        <div class="monitor-card-head">
          <span class="monitor-card-title">库表概览</span>
          <span class="monitor-sub">
            共 {{ tableTotal }} 张表
          </span>
        </div>
        <div class="schema-tags">
          <el-tag v-for="item in tableCounts" :key="item.schema" type="info">
            {{ item.schema }}：{{ item.count }}
          </el-tag>
        </div>
        <el-table :data="topRows" size="small" style="width: 100%" max-height="240">
          <el-table-column prop="schema" label="库" min-width="120" show-overflow-tooltip />
          <el-table-column prop="table" label="表" min-width="140" show-overflow-tooltip />
          <el-table-column prop="rows" label="行数（近似）" width="110" />
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { Refresh, Monitor } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

interface KV {
  label: string
  value: string | number
}

const loading = ref(false)
const data = ref<any>(null)
const autoRefresh = ref(false)

// 人类可读字节
const humanSize = (bytes: number) => {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`
}

// 运行时长格式化：1d 2h 3m
const formatUptime = (seconds: number) => {
  if (!seconds || seconds < 0) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const parts: string[] = []
  if (days) parts.push(`${days}d`)
  if (hours) parts.push(`${hours}h`)
  if (minutes) parts.push(`${minutes}m`)
  if (!days && !hours) parts.push(`${Math.floor(seconds % 60)}s`)
  return parts.join(' ')
}

const databaseRows = computed<KV[]>(() => {
  const d = data.value?.database
  if (!d) return []
  return [
    { label: 'Ping 耗时', value: `${d.ping_ms ?? 0} ms` },
    { label: '打开连接', value: d.open_connections ?? 0 },
    { label: '使用中', value: d.in_use ?? 0 },
    { label: '空闲', value: d.idle ?? 0 },
    { label: '等待次数', value: d.wait_count ?? 0 },
    { label: '等待耗时', value: `${d.wait_duration_ms ?? 0} ms` },
    { label: '最大连接数', value: d.max_open_connections ?? 0 },
    { label: '空闲关闭数', value: d.max_idle_closed ?? 0 }
  ]
})

const redisRows = computed<KV[]>(() => {
  const r = data.value?.redis
  if (!r) return []
  const rows: KV[] = [
    { label: 'Ping 耗时', value: `${r.ping_ms ?? 0} ms` },
    { label: '占用内存', value: humanSize(r.used_memory_bytes || 0) },
    { label: '客户端连接', value: r.connected_clients ?? 0 },
    { label: '命中次数', value: r.keyspace_hits ?? 0 },
    { label: '未命中次数', value: r.keyspace_misses ?? 0 },
    { label: '每秒操作', value: r.ops_per_sec ?? 0 }
  ]
  const keyspace = r.keyspace || {}
  for (const key of Object.keys(keyspace)) {
    const item = keyspace[key] || {}
    rows.push({ label: `键空间 ${key}`, value: `${item.keys ?? 0} 键 / ${item.expires ?? 0} 过期` })
  }
  if (r.pool) {
    rows.push({
      label: '连接池',
      value: `总 ${r.pool.total_conns ?? 0} / 空闲 ${r.pool.idle_conns ?? 0} / 陈旧 ${r.pool.stale_conns ?? 0}`
    })
    rows.push({
      label: '连接池命中',
      value: `${r.pool.hits ?? 0} / 未命中 ${r.pool.misses ?? 0} / 超时 ${r.pool.timeouts ?? 0}`
    })
  }
  return rows
})

const processRows = computed<KV[]>(() => {
  const p = data.value?.process
  if (!p) return []
  return [
    { label: '运行时长', value: formatUptime(p.uptime_seconds || 0) },
    { label: 'Goroutines', value: p.goroutines ?? 0 },
    { label: 'Go 版本', value: p.go_version || '-' },
    { label: '内存分配', value: humanSize(p.mem_alloc_bytes || 0) },
    { label: '系统内存', value: humanSize(p.mem_sys_bytes || 0) },
    { label: '堆内存', value: humanSize(p.mem_heap_inuse_bytes || 0) },
    { label: 'GC 次数', value: p.gc_count ?? 0 }
  ]
})

const configRows = computed<KV[]>(() => {
  const c = data.value?.config
  if (!c) return []
  return [
    { label: '服务地址', value: c.server || '-' },
    { label: '日志目录', value: c.log_dir || '-' },
    { label: '上传目录', value: c.upload_dir || '-' },
    { label: '媒体目录', value: c.media_dir || '-' }
  ]
})

const logFiles = computed<any[]>(() => data.value?.logs || [])

const tableCounts = computed<{ schema: string; count: number }[]>(() => {
  const counts = data.value?.tables?.counts || {}
  return Object.keys(counts).map((schema) => ({ schema, count: counts[schema] || 0 }))
})

const tableTotal = computed(() => tableCounts.value.reduce((sum, item) => sum + item.count, 0))

const topRows = computed<any[]>(() => data.value?.tables?.top_rows || [])

// 30s 自动刷新（组件卸载或关闭开关时清除定时器）
let timer: number | null = null
watch(autoRefresh, (on) => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (on) {
    timer = window.setInterval(loadMonitor, 30000)
  }
})

onMounted(() => {
  loadMonitor()
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const loadMonitor = async () => {
  loading.value = true
  try {
    data.value = await adminAPI.getSystemMonitor()
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-monitor {
  min-height: 100%;
}

.card-head {
  margin-bottom: 20px;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.monitor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 16px;
}

.monitor-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.monitor-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.monitor-card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.monitor-sub {
  font-size: 12px;
  color: var(--text-muted);
}

.monitor-error {
  font-size: 12px;
  color: var(--color-danger);
  word-break: break-all;
}

.kv-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kv-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
}

.kv-label {
  color: var(--text-secondary);
}

.kv-value {
  color: var(--text-primary);
  font-weight: 500;
  text-align: right;
}

.break-all {
  word-break: break-all;
}

.schema-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>