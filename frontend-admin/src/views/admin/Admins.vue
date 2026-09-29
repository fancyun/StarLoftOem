<template>
  <div class="admin-admins page-fill">
    <div class="card card-fill">
      <div class="card-head">
        <h3 class="section-title">
          <el-icon><UserFilled /></el-icon>
          员工管理
        </h3>
        <div class="head-right">
          <span class="head-tip">员工与销售均为后台账号，员工自动获得销售推广身份</span>
          <el-button v-if="canManage" type="primary" @click="handleCreate">新增账号</el-button>
        </div>
      </div>

      <el-table :data="list" class="table-fill" height="100%" style="width: 100%" v-loading="loading">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column label="昵称" width="140">
          <template #default="{ row }">{{ row.nickname || '-' }}</template>
        </el-table-column>
        <el-table-column label="权限" min-width="280">
          <template #default="{ row }">
            <el-tag v-if="row.is_super" type="danger" size="small" style="margin-right: 6px">超级管理员</el-tag>
            <span v-else-if="!row.permissions.length" class="perm-empty">未分配任何权限</span>
            <el-tag v-for="code in row.permissions" :key="code" size="small" class="perm-tag">
              {{ permissionLabel(code) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最后登录" width="170">
          <template #default="{ row }">{{ row.last_login_at ? formatDateTime(row.last_login_at) : '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="200">
          <template #default="{ row }">
            <template v-if="canManage">
              <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
              <el-button
                v-if="row.id !== 1"
                link
                :type="row.status === 1 ? 'danger' : 'success'"
                @click="handleToggleStatus(row)"
              >
                {{ row.status === 1 ? '停用' : '启用' }}
              </el-button>
              <el-button link @click="handleResetPassword(row)">重置密码</el-button>
            </template>
            <span v-else>-</span>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :total="pagination.total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="loadList"
        @size-change="loadList"
        style="margin-top: 20px; justify-content: flex-end"
      />
    </div>

    <!-- 新增账号 -->
    <el-dialog v-model="createVisible" title="新增账号" width="620px" :close-on-click-modal="false">
      <el-form :model="createForm" :rules="createRules" ref="createFormRef" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="登录用户名（至少 3 位）" maxlength="50" />
        </el-form-item>
        <el-form-item label="初始密码" prop="password">
          <el-input v-model="createForm.password" type="password" placeholder="至少 6 位" show-password maxlength="50" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="createForm.nickname" placeholder="显示名称（可选，缺省用用户名）" maxlength="50" />
        </el-form-item>
        <el-form-item label="权限">
          <PermissionPicker v-model="createForm.permissions" />
          <div class="form-tip">默认「全部权限」即管理员；也可只勾某分组的「全部」或具体页面，未勾选的菜单与接口对该账号不可用</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreateSubmit">创建</el-button>
      </template>
    </el-dialog>

    <!-- 编辑账号 -->
    <el-dialog v-model="editVisible" title="编辑账号" width="620px" :close-on-click-modal="false">
      <el-form :model="editForm" label-width="90px">
        <el-form-item label="用户名">
          <el-input :value="editForm.username" disabled />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="editForm.nickname" placeholder="显示名称" maxlength="50" />
        </el-form-item>
        <el-form-item v-if="!editForm.isSuperAdmin" label="权限">
          <PermissionPicker v-model="editForm.permissions" />
          <div class="form-tip">至少保留一名拥有「员工管理」权限的账号</div>
        </el-form-item>
        <el-form-item v-else label="权限">
          <el-tag type="danger">超级管理员（权限不可修改）</el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleEditSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { UserFilled } from '@element-plus/icons-vue'
import { adminAPI } from '@/api'
import { useAdminStore } from '@/stores/admin'
import { PERMISSION_ALL, PERMISSION_LABELS } from '@/permissions'
import PermissionPicker from '@/components/PermissionPicker.vue'
import { formatDateTime } from '@/utils/format'

interface AdminAccount {
  id: number
  username: string
  nickname: string
  status: number
  permissions: string[]
  is_super: boolean
  last_login_at?: string
  created_at: string
}

const adminStore = useAdminStore()
// 写权限：后端对员工管理的新增/编辑/启停/重置密码要求 sys.admins.write（非仅超管）
const canManage = computed(() => adminStore.has('sys.admins.write'))
const loading = ref(false)
const submitting = ref(false)
const list = ref<AdminAccount[]>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })

// 权限码展示名（含 all 与分组通配码）
const permissionLabel = (code: string) => PERMISSION_LABELS[code] || code

// ---------- 新增 ----------
const createVisible = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  username: '',
  password: '',
  nickname: '',
  permissions: [PERMISSION_ALL] as string[]
})

const createRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, message: '用户名至少 3 位', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入初始密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 位', trigger: 'blur' }
  ]
}

const handleCreate = () => {
  createForm.username = ''
  createForm.password = ''
  createForm.nickname = ''
  createForm.permissions = [PERMISSION_ALL] // 默认全部权限（默认即为管理员）
  createVisible.value = true
  createFormRef.value?.clearValidate()
}

const handleCreateSubmit = async () => {
  if (!createFormRef.value) return
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      await adminAPI.createAdmin({
        username: createForm.username.trim(),
        password: createForm.password,
        nickname: createForm.nickname.trim() || undefined,
        permissions: createForm.permissions
      })
      ElMessage.success('创建成功')
      createVisible.value = false
      loadList()
    } catch {
      // 错误消息已由请求拦截器统一提示
    } finally {
      submitting.value = false
    }
  })
}

// ---------- 编辑 ----------
const editVisible = ref(false)
const editForm = reactive({
  id: 0,
  username: '',
  nickname: '',
  permissions: [] as string[],
  isSuperAdmin: false
})

const handleEdit = (row: AdminAccount) => {
  editForm.id = row.id
  editForm.username = row.username
  editForm.nickname = row.nickname || ''
  editForm.permissions = [...row.permissions] // 按原样回显（all / 分组通配码不展开，保持可读）
  editForm.isSuperAdmin = row.is_super
  editVisible.value = true
}

const handleEditSubmit = async () => {
  submitting.value = true
  try {
    const data: { nickname: string; permissions?: string[] } = { nickname: editForm.nickname.trim() }
    if (!editForm.isSuperAdmin) {
      data.permissions = editForm.permissions
    }
    await adminAPI.updateAdmin(editForm.id, data)
    ElMessage.success('保存成功')
    editVisible.value = false
    loadList()
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    submitting.value = false
  }
}

// ---------- 启停 / 重置密码 ----------
const handleToggleStatus = async (row: AdminAccount) => {
  const action = row.status === 1 ? '停用' : '启用'
  const tip =
    row.status === 1
      ? `确认停用账号「${row.nickname || row.username}」？停用后其名下推广客户后续消费不再计提成。`
      : `确认启用账号「${row.nickname || row.username}」？`
  try {
    await ElMessageBox.confirm(tip, `${action}账号`, {
      confirmButtonText: action,
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  try {
    await adminAPI.updateAdmin(row.id, { status: row.status === 1 ? 0 : 1 })
    ElMessage.success(`已${action}`)
    loadList()
  } catch {
    // 错误消息已由请求拦截器统一提示
  }
}

const handleResetPassword = (row: AdminAccount) => {
  ElMessageBox.prompt(`请输入账号「${row.username}」的新密码`, '重置密码', {
    confirmButtonText: '重置',
    cancelButtonText: '取消',
    inputType: 'password',
    inputPattern: /^.{6,}$/,
    inputErrorMessage: '密码至少 6 位'
  })
    .then(async ({ value }) => {
      await adminAPI.resetAdminPassword(row.id, value)
      ElMessage.success('密码已重置')
    })
    .catch(() => {})
}

const loadList = async () => {
  loading.value = true
  try {
    const res: any = await adminAPI.listAdmins({ page: pagination.page, page_size: pagination.pageSize })
    list.value = res.list || []
    pagination.total = res.total || 0
  } catch {
    // 错误消息已由请求拦截器统一提示
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadList()
})
</script>

<style scoped>
.admin-admins {
  min-height: 100%;
}

.head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.head-tip {
  font-size: 13px;
  color: var(--text-muted);
}

.perm-tag {
  margin: 2px 6px 2px 0;
}

.perm-empty {
  color: var(--text-muted);
  font-size: 13px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  line-height: 1.6;
}
</style>