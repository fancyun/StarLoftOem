<template>
  <div class="admin-users page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><User /></el-icon>
          用户管理
        </h3>
        <el-button v-if="canWrite" type="primary" @click="showRegisterDialog">人工注册</el-button>
      </div>

      <div class="filter-bar">
        <el-input
          v-model="filters.phone"
          placeholder="搜索手机号/用户名"
          style="width: 200px"
          clearable
          @clear="handleSearch"
        >
          <template #append>
            <el-button icon="Search" @click="handleSearch" />
          </template>
        </el-input>

        <el-select v-model="filters.realnameStatus" placeholder="实名状态" clearable style="width: 150px" @change="handleSearch">
          <el-option label="未实名" :value="0" />
          <el-option label="个人实名" :value="1" />
          <el-option label="企业实名" :value="2" />
        </el-select>

        <el-date-picker
          v-model="filters.dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          @change="handleSearch"
        />
      </div>

      <el-table :data="users" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="用户ID" width="80" />
        <el-table-column prop="username" label="用户名" width="130">
          <template #default="{ row }">{{ row.username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" width="140" />
        <el-table-column prop="balance" label="余额" width="100">
          <template #default="{ row }">¥{{ row.balance }}</template>
        </el-table-column>
        <el-table-column prop="realname_status" label="实名状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.realname_status ? 'success' : 'info'">
              {{ realnameStatusText(row.realname_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">
              {{ row.status === 1 ? '正常' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="180">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" width="180">
          <template #default="{ row }">{{ row.last_login_at ? formatDateTime(row.last_login_at) : '-' }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="150">
          <template #default="{ row }">
            <el-button size="small" type="primary" link @click="goDetail(row)">详情</el-button>
            <el-button size="small" type="primary" link @click="openPrices(row)">单价</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadUsers"
        @size-change="loadUsers"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>

    <!-- 手动注册用户对话框 -->
    <el-dialog v-model="registerDialogVisible" title="手动注册用户" width="500px">
      <el-form :model="registerForm" :rules="registerRules" ref="registerFormRef" label-width="100px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="registerForm.username" placeholder="英文/数字/下划线，3-32位" maxlength="32" />
        </el-form-item>
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="registerForm.phone" placeholder="请输入11位手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="registerForm.password" type="password" placeholder="请输入密码（至少6位）" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="registerDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleRegisterSubmit" :loading="registerLoading">确定注册</el-button>
      </template>
    </el-dialog>

    <!-- 自定义单价对话框（仅账户实名两档；人脸核验/短信的定向定价在各产品「产品配置」页维护） -->
    <el-dialog v-model="priceDialogVisible" title="自定义单价（账户实名）" width="620px">
      <div v-loading="priceLoading" style="min-height: 120px">
        <el-descriptions :column="1" border style="margin-bottom: 16px">
          <el-descriptions-item label="用户">
            ID {{ priceUser?.id }}（{{ priceUser?.phone || '-' }}）
          </el-descriptions-item>
        </el-descriptions>
        <el-table :data="priceRows" style="width: 100%">
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
                placeholder="默认"
                controls-position="right"
                style="width: 100%"
              />
            </template>
          </el-table-column>
        </el-table>
        <div class="price-tip">留空表示不设置自定义单价，按平台价计费；人脸核验/短信的定向定价在该产品「产品配置」页维护。</div>
      </div>
      <template #footer>
        <el-button @click="priceDialogVisible = false">取消</el-button>
        <el-button v-if="canWrite" type="primary" :loading="priceSaving" @click="handleSavePrices">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { formatDateTime, kycServiceTargets, serviceLabel } from '@/utils/format'

const router = useRouter()
const adminStore = useAdminStore()
// 写权限：无 sys.users.write 时隐藏增改按钮（后端对非 GET 一律要求写权限）
const canWrite = computed(() => adminStore.has('sys.users.write'))

const loading = ref(false)
interface User {
  id: number
  username?: string
  phone: string
  balance: number
  realname_status: number // 0-未实名 1-个人实名(kyc) 2-企业实名(kyb)
  created_at: string
  last_login_at?: string
}

const users = ref<User[]>([])
const registerDialogVisible = ref(false)
const registerLoading = ref(false)
const registerFormRef = ref<FormInstance>()

const registerForm = reactive({
  username: '',
  phone: '',
  password: ''
})

const registerRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]{3,32}$/, message: '用户名仅支持英文、数字、下划线，长度3-32位', trigger: 'blur' }
  ],
  phone: [
    { required: true, message: '请输入手机号', trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

const filters = reactive({
  phone: '',
  realnameStatus: null as number | null,
  dateRange: null as [string, string] | null
})

// 实名状态展示：0-未实名 1-个人实名(kyc) 2-企业实名(kyb)
const realnameStatusText = (status: number) => {
  const map: Record<number, string> = { 1: '个人实名', 2: '企业实名' }
  return map[status] || '未实名'
}

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

onMounted(() => {
  loadUsers()
})

const loadUsers = async () => {
  loading.value = true
  try {
    const response: any = await adminAPI.getUsers({
      page: pagination.page,
      page_size: pagination.pageSize,
      phone: filters.phone,
      realname_status: filters.realnameStatus ?? undefined,
      start_date: filters.dateRange?.[0],
      end_date: filters.dateRange?.[1]
    })
    // 响应拦截器已经返回了 data，所以直接使用 response.list
    users.value = response.list || []
    pagination.total = response.total || 0
  } catch (error: any) {
    console.error('加载用户列表失败:', error)
    ElMessage.error(error.response?.data?.message || '加载用户列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  pagination.page = 1
  loadUsers()
}

const goDetail = (row: any) => {
  router.push(`/sys/users/${row.id}`)
}

const showRegisterDialog = () => {
  registerForm.phone = ''
  registerForm.password = ''
  registerDialogVisible.value = true
  // 清空表单验证
  registerFormRef.value?.clearValidate()
}

const handleRegisterSubmit = async () => {
  if (!registerFormRef.value) return

  await registerFormRef.value.validate(async (valid) => {
    if (!valid) return

    registerLoading.value = true
    try {
      const data: any = {
        username: registerForm.username,
        phone: registerForm.phone,
        password: registerForm.password
      }

      const response: any = await adminAPI.registerUser(data)

      ElMessage.success('用户注册成功')
      registerDialogVisible.value = false

      // 显示注册结果信息
      ElMessageBox.alert(
        `<div style="line-height: 1.8;">
          <p><strong>用户名：</strong>${response.username || '-'}</p>
          <p><strong>手机号：</strong>${response.phone}</p>
          <p><strong>用户ID：</strong>${response.id}</p>
        </div>`,
        '注册成功',
        {
          dangerouslyUseHTMLString: true,
          confirmButtonText: '确定'
        }
      )

      // 刷新用户列表
      loadUsers()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || '注册失败')
    } finally {
      registerLoading.value = false
    }
  })
}

// ===== 自定义单价（仅账户实名两档；人脸核验/短信的定向定价在各产品配置页） =====
const priceDialogVisible = ref(false)
const priceLoading = ref(false)
const priceSaving = ref(false)
const priceUser = ref<any>(null)
const priceRows = ref<{ target: string; label: string; platform: number; effective: number; value?: number }[]>([])
const originalTargets = ref<string[]>([])

const openPrices = async (row: any) => {
  priceUser.value = row
  priceRows.value = []
  originalTargets.value = []
  priceDialogVisible.value = true
  priceLoading.value = true
  try {
    const res: any = await adminAPI.getUserPrices(row.id)
    const services: any[] = res.services || []
    originalTargets.value = services
      .filter((s: any) => s.custom_price !== null && s.custom_price !== undefined)
      .map((s: any) => s.target)
    priceRows.value = kycServiceTargets.map((target) => {
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
  if (!priceUser.value) return
  const items = priceRows.value
    .filter((row) => typeof row.value === 'number')
    .map((row) => ({ price_type: 'unit' as const, target: row.target, price: Number(row.value) }))
  const savedTargets = new Set(items.map((item) => item.target))
  const deleted = originalTargets.value
    .filter((target) => !savedTargets.has(target))
    .map((target) => ({ price_type: 'unit' as const, target }))
  priceSaving.value = true
  try {
    await adminAPI.setUserPrices(priceUser.value.id, { items, deleted })
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
.price-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 12px;
}
</style>
