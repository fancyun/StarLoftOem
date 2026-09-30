<template>
  <div class="admin-uploads page-fill">
    <div class="card card-fill">
      <h3 class="section-title">
        <el-icon><Picture /></el-icon>
        上传文件
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
        <el-input
          v-model="filters.filePath"
          placeholder="文件路径"
          style="width: 240px"
          clearable
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        >
          <template #append>
            <el-button icon="Search" @click="handleSearch" />
          </template>
        </el-input>
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
        <el-table-column prop="file_path" label="文件路径" min-width="260" show-overflow-tooltip />
        <el-table-column label="大小" width="110">
          <template #default="{ row }">{{ humanSize(row.file_size) }}</template>
        </el-table-column>
        <el-table-column label="上传时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="100">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="preview(row)">预览</el-button>
          </template>
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

    <!-- 图片预览 -->
    <el-dialog v-model="previewVisible" title="文件预览" width="760px" top="6vh">
      <div class="preview-body" v-loading="previewLoading">
        <el-image
          v-if="previewUrl"
          :src="previewUrl"
          fit="contain"
          class="preview-img"
          :preview-src-list="[previewUrl]"
          preview-teleported
        >
          <template #placeholder><div class="preview-tip">加载中…</div></template>
          <template #error><div class="preview-tip">加载失败</div></template>
        </el-image>
        <div v-else-if="!previewLoading" class="preview-tip">无法加载该文件</div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { Picture } from '@element-plus/icons-vue'
import { adminAPI, loadAuthImage } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const records = ref<any[]>([])

const filters = reactive({
  userId: '',
  filePath: '',
  dateRange: null as [string, string] | null
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const previewVisible = ref(false)
const previewLoading = ref(false)
const previewUrl = ref('')

// 受保护图片：以 admin token 拉取 blob → objectURL；关闭弹窗时释放
const preview = async (row: any) => {
  previewVisible.value = true
  previewLoading.value = true
  previewUrl.value = ''
  try {
    previewUrl.value = await loadAuthImage(`/uploads/${row.file_path}`)
  } catch {
    previewUrl.value = ''
  } finally {
    previewLoading.value = false
  }
}

watch(previewVisible, (visible) => {
  if (!visible && previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = ''
  }
})

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

onMounted(() => {
  loadRecords()
})

const handleSearch = () => {
  pagination.page = 1
  loadRecords()
}

const loadRecords = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getUploads({
      page: pagination.page,
      page_size: pagination.pageSize,
      user_id: filters.userId || undefined,
      file_path: filters.filePath || undefined,
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
.admin-uploads {
  min-height: 100%;
}

.preview-body {
  min-height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-img {
  max-width: 100%;
  max-height: 60vh;
  cursor: zoom-in;
}

.preview-tip {
  font-size: 13px;
  color: var(--text-muted);
}
</style>