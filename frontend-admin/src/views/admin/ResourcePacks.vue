<template>
  <div class="admin-packs page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><Box /></el-icon>
          资源包管理 · {{ productLabel }}
        </h3>
        <div class="head-actions">
          <el-button v-if="canGrantTestPack" @click="handleGrantTestPack">发放测试包</el-button>
          <el-button v-if="canWrite" type="primary" @click="handleCreate">新增资源包</el-button>
        </div>
      </div>

      <!-- 页内二级菜单：人脸核验分有源/无源，短信分验证码/通知与营销 -->
      <el-tabs v-model="subProduct" class="pack-tabs">
        <template v-if="product === 'fv'">
          <el-tab-pane label="有源（fv_auth）" name="fv_auth" />
          <el-tab-pane label="无源（fv_self）" name="fv_self" />
        </template>
        <template v-else>
          <el-tab-pane label="验证码/通知短信" name="sms" />
          <el-tab-pane label="营销短信" name="sms_marketing" />
        </template>
      </el-tabs>

      <el-table :data="packs" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column label="适用类型" width="150">
          <template #default="{ row }">{{ productLabelOf(row.product) }}</template>
        </el-table-column>
        <el-table-column label="数量" width="100">
          <template #default="{ row }">{{ row.total_count }} {{ unitText() }}</template>
        </el-table-column>
        <el-table-column prop="price" label="售价" width="100">
          <template #default="{ row }">¥{{ row.price }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">
              {{ row.status === 1 ? '上架' : '下架' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="160">
          <template #default="{ row }">{{ row.description || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button v-if="canWrite" size="small" type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button v-if="canWrite" size="small" type="danger" @click="handleDelete(row)">下架</el-button>
            <span v-if="!canWrite">-</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑资源包对话框 -->
    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑资源包' : '新增资源包'" width="520px">
      <el-form :model="form" :rules="formRules" ref="formRef" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入资源包名称" maxlength="100" />
        </el-form-item>
        <el-form-item label="次数/条数" prop="total_count">
          <el-input-number
            v-model="form.total_count"
            :min="1"
            :max="maxCount"
            :step="10"
            style="width: 100%"
          />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            每个资源包包含的次数（短信为条数）
          </div>
        </el-form-item>
        <el-form-item label="售价" prop="price">
          <el-input-number
            v-model="form.price"
            :precision="2"
            :step="10"
            :min="0.01"
            :max="100000"
            style="width: 100%"
          />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            单位：元，购买时从用户余额扣费
          </div>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">上架</el-radio>
            <el-radio :value="0">下架</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入资源包描述（可选）"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="saving">确定</el-button>
      </template>
    </el-dialog>

    <!-- 发放测试包对话框（入口在产品页资源包管理：人脸核验按有源/无源分开发放） -->
    <el-dialog v-model="testPackVisible" title="发放测试包" width="460px">
      <el-form :model="testPackForm" :rules="testPackRules" ref="testPackFormRef" label-width="90px">
        <el-form-item label="发放产品">
          <el-input :value="testPackProductLabel" disabled />
        </el-form-item>
        <el-form-item label="用户" prop="user_id">
          <el-select
            v-model="testPackForm.user_id"
            filterable
            remote
            reserve-keyword
            :remote-method="searchUsers"
            :loading="userSearching"
            placeholder="输入手机号搜索用户"
            style="width: 100%"
          >
            <el-option
              v-for="u in userOptions"
              :key="u.id"
              :label="u.username ? `${u.phone}（${u.username}）` : u.phone"
              :value="u.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="发放数量" prop="count">
          <el-input-number v-model="testPackForm.count" :min="1" :max="1000" :step="1" style="width: 100%" />
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            {{ product === 'sms' ? '单位：条，默认 20 条；验证码/通知与营销测试包互不通用' : '单位：次，默认 5 次；有源与无源测试包互不通用' }}
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="testPackVisible = false">取消</el-button>
        <el-button type="primary" @click="handleTestPackSubmit" :loading="testPackLoading">确定发放</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const adminStore = useAdminStore()

// 当前一级产品（资源包按一级分区拆分页面：/sms/packs → sms、/fv/packs → fv）
const product = computed(() => (route.path.startsWith('/sms') ? 'sms' : 'fv'))
// 写权限：无对应产品资源包写权限时隐藏新增/编辑/下架按钮
const canWrite = computed(() => adminStore.has(`${product.value}.packs.write`))
// 发放测试包：需资源包写权限，且需用户管理读权限（发放时须先按手机号检索用户）
const canGrantTestPack = computed(() => canWrite.value && adminStore.has('sys.users'))

// 页内二级菜单：人脸核验（有源 fv_auth / 无源 fv_self）、短信（验证码/通知 sms / 营销 sms_marketing）
const subProduct = ref<'fv_auth' | 'fv_self' | 'sms' | 'sms_marketing'>(
  route.path.startsWith('/sms') ? 'sms' : 'fv_auth'
)

// 实际生效的类型标识（列表过滤与新建使用）
const effectiveProduct = computed(() => subProduct.value)

const productLabel = computed(() => (product.value === 'fv' ? '人脸核验' : '短信服务'))

const loading = ref(false)
const saving = ref(false)
const packs = ref<any[]>([])
const dialogVisible = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  id: 0,
  name: '',
  product: effectiveProduct.value,
  total_count: 100,
  price: 100,
  status: 1,
  description: ''
})

const productLabelOf = (p: string) => {
  if (p === 'sms') return '短信-验证码/通知'
  if (p === 'sms_marketing') return '短信-营销'
  if (p === 'fv_auth') return '人脸核验-有源'
  if (p === 'fv_self') return '人脸核验-无源'
  return p || ''
}

const unitText = () => (product.value === 'sms' ? '条' : '次')

const maxCount = computed(() => (product.value === 'sms' ? 10000000 : 100000))

const formRules: FormRules = {
  name: [
    { required: true, message: '请输入资源包名称', trigger: 'blur' },
    { min: 1, max: 100, message: '名称长度1-100个字符', trigger: 'blur' }
  ],
  total_count: [
    { required: true, message: '请输入次数/条数', trigger: 'blur' }
  ],
  price: [
    { required: true, message: '请输入售价', trigger: 'blur' }
  ]
}

onMounted(() => {
  loadPacks()
})

// 侧边栏在一级产品间切换（同一组件实例复用）时重新加载，并重置页内二级类型
watch(product, () => {
  subProduct.value = product.value === 'sms' ? 'sms' : 'fv_auth'
  resetForm()
  loadPacks()
})

// 页内二级菜单（有源/无源、验证码通知/营销）切换时重新加载
watch(subProduct, () => {
  resetForm()
  loadPacks()
})

const loadPacks = async () => {
  loading.value = true
  try {
    if (product.value === 'sms') {
      const response: any = await adminAPI.getSmsPacks(effectiveProduct.value as 'sms' | 'sms_marketing')
      packs.value = response.list || []
    } else {
      const response: any = await adminAPI.getPacks()
      packs.value = (response.list || []).filter((row: any) => row.product === effectiveProduct.value)
    }
  } catch (error: any) {
    ElMessage.error('加载资源包失败')
  } finally {
    loading.value = false
  }
}

const resetForm = () => {
  form.id = 0
  form.name = ''
  form.product = effectiveProduct.value
  form.total_count = product.value === 'sms' ? 1000 : 100
  form.price = product.value === 'sms' ? 50 : 100
  form.status = 1
  form.description = ''
}

const handleCreate = () => {
  resetForm()
  dialogVisible.value = true
  formRef.value?.clearValidate()
}

const handleEdit = (row: any) => {
  form.id = row.id
  form.name = row.name
  form.product = effectiveProduct.value
  form.total_count = row.total_count
  form.price = row.price
  form.status = row.status
  form.description = row.description || ''
  dialogVisible.value = true
  formRef.value?.clearValidate()
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (product.value === 'sms') {
        const data = {
          name: form.name.trim(),
          product: effectiveProduct.value,
          total_count: form.total_count,
          price: form.price,
          status: form.status,
          description: form.description
        }
        if (form.id) {
          await adminAPI.updateSmsPack(form.id, data)
        } else {
          await adminAPI.createSmsPack(data)
        }
      } else {
        const data = {
          name: form.name.trim(),
          product: effectiveProduct.value,
          total_count: form.total_count,
          price: form.price,
          status: form.status,
          description: form.description
        }
        if (form.id) {
          await adminAPI.updatePack(form.id, data)
        } else {
          await adminAPI.createPack(data)
        }
      }
      ElMessage.success(form.id ? '资源包更新成功' : '资源包创建成功')
      dialogVisible.value = false
      loadPacks()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || '保存失败')
    } finally {
      saving.value = false
    }
  })
}

const handleDelete = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `确认下架资源包「${row.name}」？已售出的资源包不受影响。`,
      '提示',
      {
        confirmButtonText: '确定下架',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    if (product.value === 'sms') {
      await adminAPI.deleteSmsPack(row.id)
    } else {
      await adminAPI.deletePack(row.id)
    }
    ElMessage.success('资源包已下架')
    loadPacks()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '下架失败')
    }
  }
}

// 发放测试包：发放产品取当前页产品（人脸核验页按二级菜单区分有源 fv_auth / 无源 fv_self）
const testPackVisible = ref(false)
const testPackLoading = ref(false)
const testPackFormRef = ref<FormInstance>()
const userSearching = ref(false)
const userOptions = ref<any[]>([])
const testPackForm = reactive({
  user_id: 0,
  count: 5
})
const testPackRules: FormRules = {
  user_id: [{ required: true, message: '请选择用户', trigger: 'change' }]
}
const testPackProductLabel = computed(() => productLabelOf(effectiveProduct.value))

// 按手机号检索用户（关键词为空时取前 20 条，便于直接选择）
const searchUsers = async (keyword: string) => {
  userSearching.value = true
  try {
    const response: any = await adminAPI.getUsers({
      page: 1,
      page_size: 20,
      phone: (keyword || '').trim()
    })
    userOptions.value = response.list || []
  } catch (error: any) {
    userOptions.value = []
  } finally {
    userSearching.value = false
  }
}

const handleGrantTestPack = () => {
  testPackForm.user_id = 0
  testPackForm.count = product.value === 'sms' ? 20 : 5
  testPackVisible.value = true
  testPackFormRef.value?.clearValidate()
  searchUsers('')
}

const handleTestPackSubmit = async () => {
  if (!testPackFormRef.value) return
  await testPackFormRef.value.validate(async (valid) => {
    if (!valid) return
    testPackLoading.value = true
    try {
      const result: any = product.value === 'sms'
        ? await adminAPI.grantSmsTestPack({
            user_id: testPackForm.user_id,
            product: effectiveProduct.value as 'sms' | 'sms_marketing',
            count: testPackForm.count
          })
        : await adminAPI.grantFvTestPack({
            user_id: testPackForm.user_id,
            product: effectiveProduct.value as 'fv_auth' | 'fv_self',
            count: testPackForm.count
          })
      ElMessage.success(`已发放：${result?.pack_name || '测试包'}`)
      testPackVisible.value = false
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || '发放失败')
    } finally {
      testPackLoading.value = false
    }
  })
}
</script>

<style scoped>
.admin-packs {
  min-height: 100%;
}

.pack-tabs {
  margin-bottom: 12px;
}

.head-actions {
  display: flex;
  gap: 8px;
}
</style>
