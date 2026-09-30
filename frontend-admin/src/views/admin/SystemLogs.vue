<template>
  <div class="admin-system-logs page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          系统日志
        </h3>
        <div class="log-controls">
          <el-select v-model="currentName" style="width: 160px" @change="loadFile">
            <el-option v-for="f in files" :key="f.name" :label="f.name" :value="f.name" />
          </el-select>
          <el-select v-model="lines" style="width: 120px" @change="loadFile">
            <el-option v-for="n in lineOptions" :key="n" :label="`${n} 行`" :value="n" />
          </el-select>
          <el-input
            v-model="keyword"
            placeholder="关键词（回车）"
            style="width: 200px"
            clearable
            @keyup.enter="loadFile"
            @clear="loadFile"
          />
          <el-button type="primary" :icon="Refresh" @click="loadFile">刷新</el-button>
        </div>
      </div>

      <div class="log-meta" v-if="logView">
        <span>文件大小：{{ humanSize(logView.size_bytes) }}</span>
        <span>最后写入：{{ currentFile?.modified_at ? formatDateTime(currentFile.modified_at) : '-' }}</span>
        <span>返回 {{ logView.returned }} 行</span>
        <span v-if="logView.matched">关键词命中 {{ logView.matched }} 行</span>
      </div>

      <el-alert
        v-if="logView?.truncated"
        type="warning"
        :closable="false"
        show-icon
        class="truncate-tip"
        title="仅展示文件尾部 N 行（日志只追加不轮转）；关键词过滤范围仅为本次返回窗口，全文检索请在服务器执行 grep"
      />

      <pre class="log-pre" v-loading="loading" v-html="logHtml"></pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Document, Refresh } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const lineOptions = [200, 500, 1000, 2000]

const loading = ref(false)
const files = ref<any[]>([])
const currentName = ref('access.log')
const lines = ref(200)
const keyword = ref('')
const logView = ref<any>(null)

const currentFile = computed(() => files.value.find((f) => f.name === currentName.value))

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

// 转义 HTML，避免日志内容的 <script> 等被当作标记执行
const escapeHtml = (s: string) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// 逐行转义后按关键词高亮（先转义再插入 <mark>，防止注入）
const highlight = (line: string) => {
  const safe = escapeHtml(line)
  const kw = keyword.value.trim()
  if (!kw) return safe
  return safe.replace(new RegExp(escapeRegExp(escapeHtml(kw)), 'gi'), (m) => `<mark>${m}</mark>`)
}

const logHtml = computed(() => {
  const rows: string[] = logView.value?.lines || []
  return rows.map(highlight).join('\n')
})

onMounted(async () => {
  await loadFiles()
  await loadFile()
})

const loadFiles = async () => {
  try {
    const res: any = await adminAPI.getLogFiles()
    files.value = res.files || []
    if (files.value.length && !files.value.some((f) => f.name === currentName.value)) {
      currentName.value = files.value[0].name
    }
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}

const loadFile = async () => {
  if (!currentName.value) return
  loading.value = true
  try {
    const res: any = await adminAPI.getLogFile(currentName.value, {
      lines: lines.value,
      keyword: keyword.value.trim() || undefined
    })
    logView.value = res
  } catch {
    logView.value = null
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-system-logs {
  min-height: 100%;
}

.log-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.log-meta {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-bottom: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}

.truncate-tip {
  margin-bottom: 12px;
}

.log-pre {
  flex: 1;
  min-height: 0;
  margin: 0;
  padding: 12px 16px;
  overflow: auto;
  background: var(--bg-soft);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
}

.log-pre :deep(mark) {
  background: #fff3cd;
  color: #92400e;
  padding: 0 2px;
  border-radius: 2px;
}
</style>