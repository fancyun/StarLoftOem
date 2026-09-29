<template>
  <div class="admin-settings">
    <div class="card">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Setting /></el-icon>
          系统设置
        </h3>
      </div>

      <div v-loading="loading" style="min-height: 120px">
        <el-empty v-if="!loading && settings.length === 0" description="暂无配置项" />

        <div v-for="group in groups" :key="group.category" class="setting-group">
          <h4 class="group-title">{{ group.label }}</h4>
          <el-table :data="group.items" style="width: 100%">
            <el-table-column prop="config_key" label="配置键" min-width="220" show-overflow-tooltip />
            <el-table-column label="配置值" min-width="240">
              <template #default="{ row }">
                <el-tag v-if="row.sensitive" :type="row.has_value ? 'success' : 'info'">
                  {{ row.has_value ? '已配置' : '未配置' }}
                </el-tag>
                <span v-else>{{ row.config_value || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="remark" label="说明" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">{{ row.remark || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" fixed="right" width="130">
              <template #default="{ row }">
                <el-button v-if="canWrite" link type="primary" @click="openEdit(row)">编辑</el-button>
                <el-button v-if="canWrite" link type="danger" @click="handleDelete(row)">删除</el-button>
                <span v-if="!canWrite">-</span>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" title="编辑配置" width="560px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="分组">
          <el-select v-model="form.category" style="width: 100%">
            <el-option v-for="c in categoryOptions" :key="c.value" :label="c.label" :value="c.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="配置键">
          <el-input v-model="form.config_key" disabled />
        </el-form-item>
        <el-form-item label="配置值">
          <el-input v-model="form.config_value" type="textarea" :rows="2" maxlength="2000" placeholder="请输入配置值" />
          <div v-if="isSensitive" class="form-tip">密钥类配置不回显，留空表示不修改</div>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.remark" maxlength="255" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Setting } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'

const adminStore = useAdminStore()
// 写权限：无 sys.settings.write 时隐藏编辑/删除按钮
const canWrite = computed(() => adminStore.has('sys.settings.write'))

interface SettingItem {
  config_key: string
  config_value: string
  category: string
  remark: string
  has_value: boolean
  sensitive: boolean
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const settings = ref<SettingItem[]>([])

const categoryOrder = ['starloft', 'brand', 'tencent', 'alipay', 'wechat', 'sms', 'kyc', 'payment', 'aff', 'contact', 'security', 'common']

const categoryLabels: Record<string, string> = {
  starloft: '上游平台（StarLoft）',
  brand: '品牌与域名',
  tencent: '腾讯云',
  alipay: '支付宝',
  wechat: '微信支付',
  sms: '短信服务',
  kyc: '账户实名单价',
  payment: '支付风控',
  aff: '推广分佣',
  contact: '客服联系方式',
  security: '安全密钥',
  common: '其它'
}

const categoryOptions = categoryOrder.map((value) => ({ value, label: categoryLabels[value] || value }))

const form = reactive({
  sensitive: false,
  config_key: '',
  config_value: '',
  category: 'common',
  remark: ''
})

const isSensitive = computed(() => form.sensitive)

// 按分组聚合，分组顺序按 categoryOrder，未知分组排在其后
const groups = computed(() => {
  const map = new Map<string, SettingItem[]>()
  for (const item of settings.value) {
    const items = map.get(item.category)
    if (items) {
      items.push(item)
    } else {
      map.set(item.category, [item])
    }
  }
  const indexOf = (category: string) => {
    const i = categoryOrder.indexOf(category)
    return i < 0 ? categoryOrder.length : i
  }
  return [...map.entries()]
    .sort((a, b) => indexOf(a[0]) - indexOf(b[0]))
    .map(([category, items]) => ({ category, label: categoryLabels[category] || category, items }))
})

onMounted(() => {
  loadSettings()
})

const loadSettings = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getSettings()
    settings.value = res.list || []
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

const openEdit = (row: SettingItem) => {
  form.sensitive = row.sensitive
  form.config_key = row.config_key
  form.config_value = row.sensitive ? '' : row.config_value
  form.category = row.category
  form.remark = row.remark
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!form.config_key.trim()) {
    ElMessage.warning('请输入配置键')
    return
  }
  saving.value = true
  try {
    await adminAPI.upsertSetting({
      config_key: form.config_key.trim(),
      config_value: form.config_value,
      category: form.category,
      remark: form.remark
    })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    loadSettings()
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    saving.value = false
  }
}

const handleDelete = async (row: SettingItem) => {
  try {
    await ElMessageBox.confirm(`确认删除配置「${row.config_key}」？删除后按代码内置默认值兜底，重启后端会按配置目录重新预置。`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await adminAPI.deleteSetting(row.config_key)
    ElMessage.success('已删除')
    loadSettings()
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}
</script>

<style scoped>
.admin-settings {
  min-height: 100%;
}

.setting-group {
  margin-bottom: 24px;
}

.setting-group:last-child {
  margin-bottom: 0;
}

.group-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}
</style>