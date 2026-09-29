<template>
  <div class="api-container">
    <div class="content">
      <div class="card" v-loading="pageLoading">
        <div class="card-head">
          <h3 class="section-title">
            <el-icon><Key /></el-icon>
            API 密钥
          </h3>
          <el-button
            type="primary"
            :disabled="isKycVerified < 1 || list.length >= maxKeys"
            @click="openCreate"
          >
            创建 API 密钥
          </el-button>
        </div>

        <el-alert
          v-if="!pageLoading && isKycVerified < 1"
          title="请先完成实名认证后再创建 API 密钥"
          type="warning"
          :closable="false"
          show-icon
          class="kyc-alert"
        >
          <p>完成个人实名或企业实名后，即可创建 API 密钥并配置可调用的接口权限。</p>
          <el-button type="primary" size="small" @click="$router.push('/certification')">前往实名认证</el-button>
        </el-alert>

        <template v-if="!pageLoading && isKycVerified >= 1">
          <p class="list-tip">
            每个账号最多可创建 {{ maxKeys }} 把密钥，各自独立配置权限；部分权限按接口逐项授权。
          </p>
          <el-table :data="list" stripe>
            <el-table-column prop="name" label="名称" min-width="140">
              <template #default="{ row }">{{ row.name || '未命名' }}</template>
            </el-table-column>
            <el-table-column label="API Key" min-width="260">
              <template #default="{ row }">
                <div class="value-cell">
                  <code>{{ row.api_key }}</code>
                  <el-button link @click="copyText(row.api_key)">复制</el-button>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="API Secret" min-width="240">
              <template #default="{ row }">
                <div class="value-cell">
                  <code>{{ maskSecret(row) }}</code>
                  <el-button link @click="toggleSecret(row.id)">{{ secretShown[row.id] ? '隐藏' : '显示' }}</el-button>
                  <el-button link @click="copyText(row.api_secret)">复制</el-button>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="权限" min-width="220">
              <template #default="{ row }">
                <template v-if="row.permission === 'all'">
                  <el-tag type="success">全部接口</el-tag>
                </template>
                <template v-else>
                  <el-tag
                    v-for="code in splitPermission(row.permission)"
                    :key="code"
                    class="perm-tag"
                  >
                    {{ endpointTitle(code) }}
                  </el-tag>
                </template>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="140" fixed="right">
              <template #default="{ row }">
                <el-button link @click="openEdit(row)">编辑</el-button>
                <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </template>
      </div>

      <div class="card">
        <h3 class="section-title">
          <el-icon><Document /></el-icon>
          接口文档
        </h3>
        <p class="doc-tip">完整的接口调用方法与示例请前往文档中心查看。</p>
        <el-button type="primary" @click="goApiDocs">查看 API 文档</el-button>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑 API 密钥' : '创建 API 密钥'" width="620px">
      <el-form label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="form.name" maxlength="64" placeholder="用于区分多把密钥，如：生产环境" />
        </el-form-item>
        <el-form-item label="权限范围">
          <el-radio-group v-model="form.scopeType">
            <el-radio value="all">全部接口</el-radio>
            <el-radio value="partial">部分接口</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.scopeType === 'partial'" label="可调用接口">
          <div class="endpoint-groups">
            <div v-for="group in endpointGroups" :key="group.product" class="endpoint-group">
              <div class="group-title">{{ group.title }}</div>
              <el-checkbox-group v-model="form.endpoints">
                <el-checkbox v-for="ep in group.items" :key="ep.code" :value="ep.code">
                  {{ ep.title }}
                </el-checkbox>
              </el-checkbox-group>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userAPI } from '@/api'
import { useUserStore } from '@/stores/user'
import { siteBase } from '@/utils/promotion'

const userStore = useUserStore()

const pageLoading = ref(true)
const list = ref<any[]>([])
const endpoints = ref<any[]>([])
const maxKeys = ref(10)
const isKycVerified = computed(() => (userStore.isKycVerified ? 1 : 0))

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)
const form = reactive<{ name: string; scopeType: string; endpoints: string[] }>({
  name: '',
  scopeType: 'all',
  endpoints: []
})

const secretShown = ref<Record<number, boolean>>({})

const PRODUCT_TITLES: Record<string, string> = { fv: '人脸核验', sms: '短信服务' }

const endpointGroups = computed(() => {
  const groups: { product: string; title: string; items: any[] }[] = []
  for (const ep of endpoints.value) {
    let group = groups.find((g) => g.product === ep.product)
    if (!group) {
      group = { product: ep.product, title: PRODUCT_TITLES[ep.product] || ep.product, items: [] }
      groups.push(group)
    }
    group.items.push(ep)
  }
  return groups
})

const endpointTitle = (code: string) => endpoints.value.find((e) => e.code === code)?.title || code

const splitPermission = (permission: string) =>
  (permission || '').split(',').map((s) => s.trim()).filter(Boolean)

const maskSecret = (row: any) => {
  if (!row.api_secret) return ''
  return secretShown.value[row.id] ? row.api_secret : '****' + row.api_secret.substring(row.api_secret.length - 4)
}

const toggleSecret = (id: number) => {
  secretShown.value[id] = !secretShown.value[id]
}

const formatTime = (t?: string) => (t ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '')

const copyText = (text: string) => {
  if (!text) {
    ElMessage.warning('暂无内容可复制')
    return
  }
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

const goApiDocs = () => {
  window.open(`${siteBase('portal')}/docs/fv/api/v1`, '_blank')
}

const openCreate = () => {
  editingId.value = null
  form.name = ''
  form.scopeType = 'all'
  form.endpoints = []
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  editingId.value = row.id
  form.name = row.name || ''
  form.scopeType = row.permission === 'all' ? 'all' : 'partial'
  form.endpoints = row.permission === 'all' ? [] : splitPermission(row.permission)
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (form.scopeType === 'partial' && form.endpoints.length === 0) {
    ElMessage.warning('请至少选择一个可调用的接口')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      scope_type: form.scopeType,
      endpoints: form.scopeType === 'partial' ? form.endpoints : []
    }
    if (editingId.value) {
      await userAPI.updateAPIKey(editingId.value, payload)
      ElMessage.success('API 密钥已修改')
    } else {
      await userAPI.createAPIKey(payload)
      ElMessage.success('API 密钥已创建')
    }
    dialogVisible.value = false
    await loadKeys()
  } catch (error: any) {
    ElMessage.error(error?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: any) => {
  try {
    await ElMessageBox.confirm('删除后该密钥立即失效，确定要删除吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await userAPI.deleteAPIKey(row.id)
    ElMessage.success('API 密钥已删除')
    await loadKeys()
  } catch (error: any) {
    ElMessage.error(error?.message || '删除失败')
  }
}

const loadKeys = async () => {
  const res = await userAPI.listAPIKeys()
  list.value = res.list || []
  endpoints.value = res.endpoints || []
  maxKeys.value = res.max || 10
}

const loadData = async () => {
  try {
    const profile = await userAPI.getProfile()
    userStore.setUserInfo(profile)
    if (profile.realname_status >= 1) {
      await loadKeys()
    }
  } catch (error) {
    console.error(error)
  } finally {
    pageLoading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.api-container {
  min-height: 100%;
}

.content {
  max-width: 1100px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.list-tip {
  color: var(--text-muted);
  font-size: 13px;
  margin-bottom: 12px;
}

.value-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.value-cell code {
  padding: 4px 8px;
  background: var(--bg-soft);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-sm);
  color: var(--color-primary);
  font-family: 'Courier New', monospace;
  font-size: 12px;
  word-break: break-all;
}

.perm-tag {
  margin: 2px 4px 2px 0;
}

.endpoint-groups {
  width: 100%;
}

.endpoint-group {
  margin-bottom: 8px;
}

.group-title {
  color: var(--text-muted);
  font-size: 13px;
  margin-bottom: 4px;
}

.doc-tip {
  color: var(--text-muted);
  font-size: 14px;
  margin-bottom: 16px;
}

.kyc-alert {
  margin-bottom: 8px;
}
.kyc-alert p {
  margin: 4px 0 12px;
  font-size: 13px;
  line-height: 1.6;
}
</style>