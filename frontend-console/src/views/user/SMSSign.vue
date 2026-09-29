<template>
  <div class="sms-sign-container page-fill">
    <div class="content">
      <div class="card card-fill">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Document /></el-icon>
            我的签名
          </h3>
          <el-button type="primary" @click="goCreate">提交签名</el-button>
        </div>
        <el-table :data="signs" class="table-fill" height="100%" style="width: 100%" v-loading="listLoading">
          <el-table-column prop="sign_name" label="签名内容" min-width="140" />
          <el-table-column prop="id" label="签名ID" width="90" />
          <el-table-column prop="up_sign_id" label="上游ID" width="110" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.status === 2" type="success">通过</el-tag>
              <el-tag v-else-if="row.status === 3" type="danger">驳回</el-tag>
              <el-tag v-else-if="row.status === 1" type="warning">审核中</el-tag>
              <el-tag v-else type="info">待审核</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="result_message" label="审核结果" min-width="160" />
          <el-table-column label="可见性" width="90">
            <template #default="{ row }">
              <el-tag v-if="row.is_public === 1" type="warning">公共</el-tag>
              <el-tag v-else type="info">私有</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="提交时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="210" align="center">
            <template #default="{ row }">
              <template v-if="row.is_public !== 1">
                <el-button link type="primary" :loading="queryingId === row.id" @click="handleQueryStatus(row)">
                  查询结果
                </el-button>
                <el-button
                  link
                  type="warning"
                  :disabled="row.status !== 2 && row.status !== 3"
                  @click="handleEdit(row)"
                >
                  修改
                </el-button>
              </template>
              <span v-else class="muted">平台维护</span>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-if="total > pageSize"
          v-model:current-page="page"
          :total="total"
          :page-size="pageSize"
          layout="total, prev, pager, next"
          @current-change="loadSigns"
          style="margin-top: 20px; justify-content: center"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { smsAPI } from '@/api'
import { formatDateTime } from '@/utils/format'

const router = useRouter()

const listLoading = ref(false)
const queryingId = ref<number | null>(null)
const signs = ref<any[]>([])
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const goCreate = () => {
  router.push('/sms/sign/create')
}

// 进入修改：跳转独立的修改页面（仅已通过/驳回可改，操作列按钮已按状态禁用）
const handleEdit = (row: any) => {
  router.push(`/sms/sign/edit/${row.id}`)
}

const loadSigns = async () => {
  listLoading.value = true
  try {
    const res = await smsAPI.listSigns({ page: page.value, page_size: pageSize.value })
    signs.value = res.list
    total.value = res.total
  } catch (error) {
    console.error(error)
  } finally {
    listLoading.value = false
  }
}

// 手动查询签名审核状态（调上游核对并回写本地）
const handleQueryStatus = async (row: any) => {
  queryingId.value = row.id
  try {
    const res: any = await smsAPI.querySignStatus(row.id)
    if (res?.status === 2) {
      ElMessage.success('该签名已审核通过')
    } else if (res?.status === 3) {
      ElMessage.error(res?.result_message || '该签名审核未通过')
    } else if (!res?.up_sign_id) {
      ElMessage.info('该签名尚未报备上游，无审核结果')
    } else {
      ElMessage.info(`该签名上游${res?.result_message || '审核中'}，请耐心等待`)
    }
    await loadSigns()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败，请稍后重试')
  } finally {
    queryingId.value = null
  }
}

onMounted(() => {
  loadSigns()
})
</script>

<style scoped>
.sms-sign-container {
  min-height: 0;
}

.content {
  display: flex;
  flex-direction: column;
}

.muted {
  color: var(--text-muted);
}
</style>