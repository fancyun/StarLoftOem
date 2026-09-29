<template>
  <div class="sms-template-container page-fill">
    <div class="content">
      <div class="card card-fill">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Document /></el-icon>
            我的模板
          </h3>
          <el-button type="primary" @click="goCreate">提交模板</el-button>
        </div>
        <el-table :data="templates" class="table-fill" height="100%" style="width: 100%" v-loading="listLoading">
          <el-table-column prop="template_name" label="模板名称" min-width="140" />
          <el-table-column label="类型" width="90">
            <template #default="{ row }">{{ typeText(row.template_type) }}</template>
          </el-table-column>
          <el-table-column prop="sign_name" label="关联签名" min-width="120" />
          <el-table-column prop="template_content" label="模板内容" min-width="220" show-overflow-tooltip />
          <el-table-column prop="template_id" label="模板ID" width="120" />
          <el-table-column label="状态" width="110">
            <template #default="{ row }">
              <el-tag v-if="row.status === 3" type="warning">待平台审核</el-tag>
              <el-tag v-else-if="row.status === 1" type="success">已审核</el-tag>
              <el-tag v-else-if="row.status === 2" type="danger">驳回</el-tag>
              <el-tag v-else type="info">待审核</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="说明" min-width="120" />
          <el-table-column label="操作" width="210" align="center">
            <template #default="{ row }">
              <el-button
                v-if="row.status !== 3"
                link
                type="primary"
                :loading="queryingId === row.id"
                @click="handleQueryStatus(row)"
              >
                查询结果
              </el-button>
              <el-button
                link
                type="warning"
                :disabled="row.status !== 1 && row.status !== 2"
                @click="handleEdit(row)"
              >
                修改
              </el-button>
              <el-button v-if="!row.template_id" link type="primary" @click="openFill(row)">回填模板ID</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          v-if="total > pageSize"
          v-model:current-page="page"
          :total="total"
          :page-size="pageSize"
          layout="total, prev, pager, next"
          @current-change="loadTemplates"
          style="margin-top: 20px; justify-content: center"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { smsAPI } from '@/api'

const router = useRouter()

const listLoading = ref(false)
const queryingId = ref<number | null>(null)
const templates = ref<any[]>([])
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

function typeText(type: string) {
  if (type === 'verify') return '验证码'
  if (type === 'marketing') return '营销'
  return '通知'
}

const goCreate = () => {
  router.push('/sms/template/create')
}

// 进入修改：跳转独立的修改页面（仅已审核/驳回可改，操作列按钮已按状态禁用）
const handleEdit = (row: any) => {
  router.push(`/sms/template/edit/${row.id}`)
}

const loadTemplates = async () => {
  listLoading.value = true
  try {
    const res = await smsAPI.listTemplates({ page: page.value, page_size: pageSize.value })
    templates.value = res.list
    total.value = res.total
  } catch (error) {
    console.error(error)
  } finally {
    listLoading.value = false
  }
}

// 手动查询模板审核状态（调上游核对并回写本地）
const handleQueryStatus = async (row: any) => {
  queryingId.value = row.id
  try {
    const res: any = await smsAPI.queryTemplateStatus(row.id)
    if (res?.status === 1) {
      ElMessage.success('该模板已审核通过')
    } else if (res?.status === 2) {
      ElMessage.error(res?.reason || '该模板审核未通过')
    } else if (!res?.template_id) {
      ElMessage.info('该模板尚未报备上游，无审核结果')
    } else {
      ElMessage.info('该模板仍在审核中，请耐心等待')
    }
    await loadTemplates()
  } catch (error: any) {
    ElMessage.error(error?.message || '查询失败，请稍后重试')
  } finally {
    queryingId.value = null
  }
}

const openFill = (row: any) => {
  ElMessageBox.prompt('请输入在上游控制台创建的模板 ID', '回填模板ID', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '模板 ID 不能为空'
  })
    .then(async ({ value }) => {
      await smsAPI.fillTemplateID(row.id, { template_id: value })
      ElMessage.success('模板 ID 已回填')
      loadTemplates()
    })
    .catch(() => {})
}

onMounted(() => {
  loadTemplates()
})
</script>

<style scoped>
.sms-template-container {
  min-height: 0;
}

.content {
  display: flex;
  flex-direction: column;
}
</style>