<template>
  <div class="admin-sms-replies page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><ChatDotRound /></el-icon>
          短信回复
        </h3>
        <span class="section-tip">用户回复上行内容，来自上游推送/按日拉取</span>
      </div>

      <el-table :data="replies" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="user_phone" label="用户手机号" width="130" />
        <el-table-column prop="phone" label="来源号码" width="130" />
        <el-table-column prop="content_down" label="下行内容" min-width="180" show-overflow-tooltip />
        <el-table-column prop="content_up" label="回复内容" min-width="160" show-overflow-tooltip />
        <el-table-column prop="resp_time" label="回复时间" width="170" />
        <el-table-column prop="task_id" label="任务ID" width="160" show-overflow-tooltip />
        <el-table-column prop="created_at" label="记录时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadReplies"
        @size-change="loadReplies"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { ChatDotRound } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const replies = ref<any[]>([])

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadReplies()
})

const loadReplies = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getSmsReplies({
      page: pagination.page,
      page_size: pagination.pageSize
    })
    replies.value = res.list || []
    pagination.total = res.total || 0
  } catch (error: any) {
    ElMessage.error('加载短信回复失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.admin-sms-replies {
  min-height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 0;
}

.section-tip {
  font-size: 13px;
  color: var(--text-muted);
}
</style>
