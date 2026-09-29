<template>
  <div class="admin-product-config">
    <div class="notice">修改后需重启后端服务生效，未配置项按代码内置默认值兜底。</div>

    <div class="card">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Box /></el-icon>
          产品配置 · {{ productLabel }}
        </h3>
        <div class="head-actions">
          <el-button v-if="canRead" @click="openUserPrices">用户定向定价</el-button>
          <el-button v-if="canWrite" type="primary" @click="openCreate">新增配置</el-button>
        </div>
      </div>
      <el-table :data="list" style="width: 100%" v-loading="loading">
        <el-table-column prop="config_key" label="配置键" min-width="200" />
        <el-table-column prop="config_value" label="配置值" min-width="160">
          <template #default="{ row }">{{ row.config_value || '-' }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="说明" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">{{ row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.updated_at) }}</template>
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

    <el-dialog v-model="dialogVisible" :title="form.isEdit ? '编辑产品配置' : '新增产品配置'" width="560px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="产品">
          <el-input :value="productLabel" disabled />
        </el-form-item>
        <el-form-item label="配置键">
          <el-select
            v-model="form.config_key"
            :disabled="form.isEdit"
            filterable
            allow-create
            default-first-option
            placeholder="选择或输入配置键"
            style="width: 100%"
          >
            <el-option v-for="k in knownKeys" :key="k.value" :label="k.label" :value="k.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="配置值">
          <el-select v-if="isEnabledKey" v-model="form.config_value" style="width: 100%">
            <el-option label="1 - 开放" value="1" />
            <el-option label="0 - 关闭" value="0" />
          </el-select>
          <el-input v-else v-model="form.config_value" maxlength="512" placeholder="请输入配置值" />
          <div v-if="valueTip" class="form-tip">{{ valueTip }}</div>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.remark" maxlength="255" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button v-if="canWrite" type="primary" :loading="saving" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>

    <!-- 用户定向定价：本产品服务的平台价 / 当前生效价 / 用户级自定义价（覆盖价存本产品库，保存即生效） -->
    <el-dialog v-model="priceDialogVisible" title="用户定向定价" width="640px">
      <el-form label-width="80px">
        <el-form-item label="用户">
          <el-select
            v-model="priceUserId"
            filterable
            remote
            reserve-keyword
            :remote-method="searchUsers"
            :loading="userSearching"
            placeholder="输入手机号 / 用户名搜索用户"
            style="width: 100%"
            @change="loadUserPrices"
          >
            <el-option
              v-for="u in userOptions"
              :key="u.id"
              :label="u.username ? `${u.phone}（${u.username}）` : u.phone"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <el-table :data="priceRows" style="width: 100%" v-loading="priceLoading" empty-text="请选择用户">
        <el-table-column prop="label" label="服务" min-width="110" />
        <el-table-column label="平台价" width="100">
          <template #default="{ row }">¥{{ row.platform }}</template>
        </el-table-column>
        <el-table-column label="当前生效单价" width="130">
          <template #default="{ row }">¥{{ row.effective }}</template>
        </el-table-column>
        <el-table-column label="自定义单价" width="180">
          <template #default="{ row }">
            <el-input-number
              v-model="row.value"
              :min="0"
              :precision="2"
              :step="1"
              :disabled="!canWrite"
              placeholder="默认"
              controls-position="right"
              style="width: 100%"
            />
          </template>
        </el-table-column>
      </el-table>
      <div class="form-tip">留空表示不设置自定义单价，按平台价计费；保存后立即生效于该用户在本产品的消费。</div>
      <template #footer>
        <el-button @click="priceDialogVisible = false">取消</el-button>
        <el-button
          v-if="canWrite"
          type="primary"
          :loading="priceSaving"
          :disabled="!priceUserId"
          @click="handleSavePrices"
        >保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Box } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime, fvServiceTargets, smsServiceTargets, serviceLabel } from '@/utils/format'

const adminStore = useAdminStore()

interface ProductConfigItem {
  id: number
  config_key: string
  config_value: string
  remark: string
  updated_at: string
}

// 产品配置键（与后端 config.FvProductConfigCatalog / SmsProductConfigCatalog 一致）
const knownKeyOptions: Record<'fv' | 'sms', { value: string; label: string }[]> = {
  fv: [
    { value: 'fv_auth_price', label: 'fv_auth_price（有源人脸核验单价）' },
    { value: 'fv_self_price', label: 'fv_self_price（无源人脸核验单价）' },
    { value: 'fv_auth_cost', label: 'fv_auth_cost（有源人脸核验成本单价）' },
    { value: 'fv_self_cost', label: 'fv_self_cost（无源人脸核验成本单价）' }
  ],
  sms: [
    { value: 'sms_price', label: 'sms_price（平台短信单价，元/条）' },
    { value: 'sms_cost', label: 'sms_cost（短信成本单价，元/条）' }
  ]
}

const route = useRoute()
// 当前页展示的产品由路由决定（/fv/product-config → fv，/sms/product-config → sms）
const product = computed<'fv' | 'sms'>(() => (route.meta.product === 'sms' ? 'sms' : 'fv'))
const productLabel = computed(() => (product.value === 'fv' ? '人脸核验' : '短信服务'))
// 写权限：无对应产品配置写权限时隐藏新增/编辑/删除按钮
const canWrite = computed(() => adminStore.has(`${product.value}.product_config.write`))
// 读权限：无对应产品配置读权限时隐藏「用户定向定价」入口
const canRead = computed(() => adminStore.has(`${product.value}.product_config`))

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const list = ref<ProductConfigItem[]>([])

const form = reactive({
  isEdit: false,
  config_key: '',
  config_value: '',
  remark: ''
})

const knownKeys = computed(() => knownKeyOptions[product.value])
const isEnabledKey = computed(() => form.config_key.endsWith('_enabled'))
const valueTip = computed(() => {
  if (isEnabledKey.value) return '1-开放 0-关闭'
  if (form.config_key.endsWith('_cost')) return '单位：元（成本不计入售价，仅用于按利润计提推广提成）'
  if (form.config_key.endsWith('_price')) return '单位：元'
  return ''
})

onMounted(() => {
  loadConfigs()
})

// 同一组件复用于 fv/sms 两个路由，产品切换时重新加载
watch(product, () => {
  priceDialogVisible.value = false
  loadConfigs()
})

const loadConfigs = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.getProductConfigs(product.value)
    list.value = res[product.value] || []
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  form.isEdit = false
  form.config_key = ''
  form.config_value = ''
  form.remark = ''
  dialogVisible.value = true
}

const openEdit = (row: ProductConfigItem) => {
  form.isEdit = true
  form.config_key = row.config_key
  form.config_value = row.config_value
  form.remark = row.remark
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!form.config_key) {
    ElMessage.warning('请选择或输入配置键')
    return
  }
  saving.value = true
  try {
    await adminAPI.upsertProductConfig({
      product: product.value,
      config_key: form.config_key,
      config_value: form.config_value,
      remark: form.remark
    })
    ElMessage.success('保存成功')
    dialogVisible.value = false
    loadConfigs()
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    saving.value = false
  }
}

const handleDelete = async (row: ProductConfigItem) => {
  try {
    await ElMessageBox.confirm(`确认删除配置「${row.config_key}」？删除后该项按代码内置默认值兜底。`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await adminAPI.deleteProductConfig(product.value, row.config_key)
    ElMessage.success('已删除')
    loadConfigs()
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}

// ===== 用户定向定价（覆盖价存本产品库，服务清单随产品切换） =====
const priceDialogVisible = ref(false)
const priceLoading = ref(false)
const priceSaving = ref(false)
const userSearching = ref(false)
const userOptions = ref<any[]>([])
const priceUserId = ref<number>()
const priceRows = ref<{ target: string; label: string; platform: number; effective: number; value?: number }[]>([])
const originalTargets = ref<string[]>([])

const productServiceTargets = computed(() => (product.value === 'fv' ? fvServiceTargets : smsServiceTargets))

const openUserPrices = () => {
  priceUserId.value = undefined
  priceRows.value = []
  originalTargets.value = []
  userOptions.value = []
  priceDialogVisible.value = true
  searchUsers('')
}

// 按手机号/用户名检索用户（关键词为空时取前 20 条）
const searchUsers = async (keyword: string) => {
  userSearching.value = true
  try {
    const res: any = await adminAPI.getProductPriceUsers({
      product: product.value,
      keyword: (keyword || '').trim()
    })
    userOptions.value = res.list || []
  } catch {
    userOptions.value = []
  } finally {
    userSearching.value = false
  }
}

const loadUserPrices = async () => {
  if (!priceUserId.value) return
  priceLoading.value = true
  try {
    const res: any = await adminAPI.getProductPrices({
      product: product.value,
      user_id: priceUserId.value
    })
    const services: any[] = res.services || []
    originalTargets.value = services
      .filter((s: any) => s.custom_price !== null && s.custom_price !== undefined)
      .map((s: any) => s.target)
    priceRows.value = productServiceTargets.value.map((target) => {
      const item = services.find((s: any) => s.target === target)
      return {
        target,
        label: serviceLabel(target),
        platform: item?.platform_price ?? 0,
        effective: item?.effective_price ?? 0,
        value: item?.custom_price ?? undefined
      }
    })
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    priceLoading.value = false
  }
}

const handleSavePrices = async () => {
  if (!priceUserId.value) return
  const items = priceRows.value
    .filter((row) => typeof row.value === 'number')
    .map((row) => ({ price_type: 'unit' as const, target: row.target, price: Number(row.value) }))
  const savedTargets = new Set(items.map((item) => item.target))
  const deleted = originalTargets.value
    .filter((target) => !savedTargets.has(target))
    .map((target) => ({ price_type: 'unit' as const, target }))
  priceSaving.value = true
  try {
    await adminAPI.setProductPrices({
      product: product.value,
      user_id: priceUserId.value,
      items,
      deleted
    })
    ElMessage.success('单价已保存')
    priceDialogVisible.value = false
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    priceSaving.value = false
  }
}
</script>

<style scoped>
.admin-product-config {
  min-height: 100%;
}

.notice {
  margin-bottom: 16px;
  padding: 10px 14px;
  border-radius: var(--radius-md);
  background: var(--bg-soft);
  color: var(--text-secondary);
  font-size: 13px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

.head-actions {
  display: flex;
  gap: 8px;
}
</style>