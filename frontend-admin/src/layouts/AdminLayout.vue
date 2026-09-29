<template>
  <div class="admin-layout">
    <!-- 顶部 Header（一级，全宽置顶，与用户端一致） -->
    <header class="layout-header">
      <div class="header-left">
        <router-link to="/sys/dashboard" class="header-title">
          <img class="brand-logo" src="/logo.jpg" alt="星楼网络" />
          <span>管理后台</span>
        </router-link>
      </div>
      <div class="header-right">
        <el-dropdown @command="handleCommand">
          <span class="user-dropdown">
            <el-icon><UserFilled /></el-icon>
            <span>{{ adminName }}</span>
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="changePassword">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <div class="layout-body">
      <!-- 左侧侧边栏（顶部为一级分区切换，下方按分区渲染各自菜单，互不互通） -->
      <aside class="layout-sidebar">
        <div class="section-switch-wrap">
          <el-dropdown class="section-switch-dropdown" @command="handleSectionSwitch">
            <span class="section-switch">
              <el-icon><Menu /></el-icon>
              <span>{{ currentSection.label }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="s in visibleSectionOptions"
                  :key="s.key"
                  :command="s.key"
                  :disabled="s.key === section"
                >
                  {{ s.label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <nav class="sidebar-nav">
          <template v-for="item in currentMenu" :key="item.type === 'group' ? item.label : item.to">
            <router-link
              v-if="item.type === 'link'"
              :to="item.to"
              class="sidebar-item"
              :class="{ active: isLinkActive(item) }"
            >
              <svg class="sidebar-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="iconPaths[item.icon] || ''"></svg>
              <span>{{ item.label }}</span>
            </router-link>
            <div v-else class="sidebar-group">
              <div
                class="sidebar-group-head"
                :class="{ active: isGroupActive(item) }"
                @click="toggleGroup(item.label)"
              >
                <svg class="sidebar-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="iconPaths[item.icon] || ''"></svg>
                <span>{{ item.label }}</span>
                <el-icon class="group-caret" :class="{ open: openGroups.has(item.label) }"><ArrowDown /></el-icon>
              </div>
              <el-collapse-transition>
                <div v-show="openGroups.has(item.label)" class="sidebar-subnav">
                  <router-link
                    v-for="child in item.children"
                    :key="child.to"
                    :to="child.to"
                    class="sidebar-item"
                    :class="{ active: isLinkActive(child) }"
                  >
                    <span>{{ child.label }}</span>
                  </router-link>
                </div>
              </el-collapse-transition>
            </div>
          </template>
        </nav>
      </aside>

      <!-- 右侧主内容区 -->
      <main class="layout-main">
        <router-view />
      </main>
    </div>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="420px" :close-on-click-modal="false">
      <el-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-width="80px"
      >
        <el-form-item label="旧密码" prop="old_password">
          <el-input
            v-model="passwordForm.old_password"
            type="password"
            placeholder="请输入当前密码"
            show-password
          />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input
            v-model="passwordForm.new_password"
            type="password"
            placeholder="至少6位"
            show-password
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input
            v-model="passwordForm.confirm_password"
            type="password"
            placeholder="再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="passwordLoading" @click="handleChangePassword">
          确定修改
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { UserFilled, ArrowDown, Menu } from '@element-plus/icons-vue'
import { useAdminStore } from '@/stores/admin'
import { adminAPI } from '@/api'

const router = useRouter()
const route = useRoute()
const adminStore = useAdminStore()

const adminName = computed(() => adminStore.adminInfo?.nickname || adminStore.adminInfo?.username || '管理员')

// ===== 一级分区 =====
const sectionOptions = [
  { key: 'sys', label: '平台管理' },
  { key: 'sms', label: '短信服务' },
  { key: 'fv', label: '人脸核验' }
]

const section = computed(() => {
  const s = route.path.split('/')[1]
  return s === 'sys' || s === 'sms' || s === 'fv' ? s : 'sys'
})

const currentSection = computed(
  () => sectionOptions.find((s) => s.key === section.value) || sectionOptions[0]
)

// 各分区侧边栏菜单（互不互通）；每项 permission 与后端权限码一致，无权限则整项（或整组）隐藏
const sections: Record<string, { label: string; defaultPath: string; menu: any[] }> = {
  sys: {
    label: '平台管理',
    defaultPath: '/sys/dashboard',
    menu: [
      { type: 'link', to: '/sys/dashboard', label: '数据统计', icon: 'chart', permission: 'sys.dashboard' },
      {
        type: 'group',
        label: '用户管理',
        icon: 'user',
        children: [
          { to: '/sys/users', label: '用户管理', permission: 'sys.users' },
          { to: '/sys/kyc', label: '个人实名', permission: 'sys.users' },
          { to: '/sys/kyb', label: '企业实名', permission: 'sys.users' }
        ]
      },
      {
        type: 'group',
        label: '财务管理',
        icon: 'finance',
        children: [
          { to: '/sys/finance', label: '财务统计', permission: 'sys.finance' },
          { to: '/sys/bills', label: '账单统计', permission: 'sys.finance' },
          { to: '/sys/payments', label: '支付记录', permission: 'sys.finance' }
        ]
      },
      {
        type: 'group',
        label: '推广管理',
        icon: 'promotion',
        children: [
          { to: '/sys/commissions', label: '提成记录', permission: 'sys.aff' },
          { to: '/sys/promotion/withdraws', label: '提现审核', permission: 'sys.aff' }
        ]
      },
      { type: 'link', to: '/sys/sales', label: '销售业绩', icon: 'promotion', permission: 'sys.sales' },
      { type: 'link', to: '/sys/admins', label: '员工管理', icon: 'user', permission: 'sys.admins' },
      { type: 'link', to: '/sys/settings', label: '系统设置', icon: 'config', permission: 'sys.settings' }
    ]
  },
  sms: {
    label: '短信服务',
    defaultPath: '/sms/stats',
    menu: [
      { type: 'link', to: '/sms/stats', label: '数据统计', icon: 'chart', permission: 'sms.stats' },
      { type: 'link', to: '/sms/signs', label: '签名审核', icon: 'sign', permission: 'sms.signs' },
      { type: 'link', to: '/sms/templates', label: '模板审核', icon: 'template', permission: 'sms.templates' },
      { type: 'link', to: '/sms/records', label: '发送记录', icon: 'records', permission: 'sms.records' },
      { type: 'link', to: '/sms/replies', label: '短信回复', icon: 'reply', permission: 'sms.replies' },
      { type: 'link', to: '/sms/packs', label: '资源包管理', icon: 'packs', permission: 'sms.packs' },
      { type: 'link', to: '/sms/product-config', label: '产品配置', icon: 'config', permission: 'sms.product_config' }
    ]
  },
  fv: {
    label: '人脸核验',
    defaultPath: '/fv/stats',
    menu: [
      { type: 'link', to: '/fv/stats', label: '数据统计', icon: 'chart', permission: 'fv.stats' },
      { type: 'link', to: '/fv/records', label: '认证记录', icon: 'orders', permission: 'fv.records' },
      { type: 'link', to: '/fv/packs', label: '资源包管理', icon: 'packs', permission: 'fv.packs' },
      { type: 'link', to: '/fv/product-config', label: '产品配置', icon: 'config', permission: 'fv.product_config' }
    ]
  }
}

// hasPerm 是否持有权限（权限缺省视为公开项；数组任一命中即通过）
const hasPerm = (perm?: string | string[]) => {
  if (!perm) return true
  const codes = Array.isArray(perm) ? perm : [perm]
  return codes.some((code) => adminStore.has(code))
}

// visibleMenu 按权限过滤菜单：分组内子项全部无权时整组隐藏
const visibleMenu = (key: string) => {
  const menu = sections[key]?.menu || []
  return menu
    .map((item: any) => {
      if (item.type === 'group') {
        const children = item.children.filter((child: any) => hasPerm(child.permission))
        return children.length ? { ...item, children } : null
      }
      return hasPerm(item.permission) ? item : null
    })
    .filter(Boolean) as any[]
}

// 仅展示含可见菜单的分区
const visibleSectionOptions = computed(() => sectionOptions.filter((s) => visibleMenu(s.key).length > 0))

const currentMenu = computed(() => visibleMenu(section.value))

// sectionDefaultPath 分区内第一个可见菜单的路径（用于分区切换，避免进入无权限页）
const sectionDefaultPath = (key: string) => {
  for (const item of visibleMenu(key)) {
    if (item.type === 'link') return item.to
    if (item.type === 'group' && item.children.length) return item.children[0].to
  }
  return sections[key]?.defaultPath || '/login'
}

// 菜单图标（内联 SVG 片段，受控内容）
const iconPaths: Record<string, string> = {
  chart: '<line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/>',
  user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  finance: '<line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>',
  sign: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
  template: '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>',
  records: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
  reply: '<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/>',
  orders: '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
  packs: '<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>',
  promotion: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>',
  config: '<line x1="4" y1="21" x2="4" y2="14"/><line x1="4" y1="10" x2="4" y2="3"/><line x1="12" y1="21" x2="12" y2="12"/><line x1="12" y1="8" x2="12" y2="3"/><line x1="20" y1="21" x2="20" y2="16"/><line x1="20" y1="12" x2="20" y2="3"/><line x1="1" y1="14" x2="7" y2="14"/><line x1="9" y1="8" x2="15" y2="8"/><line x1="17" y1="16" x2="23" y2="16"/>'
}

// 折叠组展开状态
const openGroups = ref<Set<string>>(new Set())

const toggleGroup = (label: string) => {
  if (openGroups.value.has(label)) {
    openGroups.value.delete(label)
  } else {
    openGroups.value.add(label)
  }
}

// exact 为 true 时仅路径完全一致才算选中（避免父级菜单抢占子路由高亮）
const isLinkActive = (item: { to: string; exact?: boolean }) => {
  if (item.exact) return route.path === item.to
  return route.path === item.to || route.path.startsWith(item.to + '/')
}

const isGroupActive = (item: { children: { to: string }[] }) => {
  return item.children.some((child) => isLinkActive(child))
}

// 分区/路径变化时自动展开包含当前页的折叠组
watch(
  [section, () => route.path],
  () => {
    for (const item of visibleMenu(section.value)) {
      if (item.type === 'group' && item.children.some((child: { to: string }) => isLinkActive(child))) {
        openGroups.value.add(item.label)
      }
    }
  },
  { immediate: true }
)

const handleSectionSwitch = (key: string) => {
  if (key !== section.value) {
    router.push(sectionDefaultPath(key))
  }
}

// ===== 修改密码 =====
const passwordDialogVisible = ref(false)
const passwordLoading = ref(false)
const passwordFormRef = ref<FormInstance>()

const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const passwordRules: FormRules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '新密码至少6位', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== passwordForm.new_password) {
          callback(new Error('两次输入的新密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return
  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return
    passwordLoading.value = true
    try {
      await adminAPI.changePassword({
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password
      })
      ElMessage.success('密码修改成功')
      passwordDialogVisible.value = false
      passwordForm.old_password = ''
      passwordForm.new_password = ''
      passwordForm.confirm_password = ''
    } catch (error) {
      // 错误消息已由请求拦截器统一提示
    } finally {
      passwordLoading.value = false
    }
  })
}

const handleCommand = (command: string) => {
  if (command === 'changePassword') {
    passwordForm.old_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
    passwordFormRef.value?.clearValidate()
    passwordDialogVisible.value = true
  } else if (command === 'logout') {
    adminStore.clearAdminInfo()
    ElMessage.success('已退出登录')
    router.push('/login')
  }
}
</script>

<style scoped>
.admin-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

/* ========== 顶部 Header（与用户端 layout-header 完全一致） ========== */
.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  padding: 0 24px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  margin-right: auto;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

/* 侧边栏顶部一级分区切换按钮 */
.section-switch-wrap {
  padding: 12px 8px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.section-switch-dropdown {
  display: block;
  width: 100%;
}

.section-switch {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid var(--border-color);
  transition: background 0.15s;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 600;
  background: var(--bg-page);
}

.section-switch:hover {
  background: var(--bg-hover);
}

.header-right {
  display: flex;
  align-items: center;
}

.user-dropdown {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  transition: background 0.15s;
  color: var(--text-secondary);
}

.user-dropdown:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

/* ========== 主体 ========== */
.layout-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* ========== 左侧侧边栏（与用户端 layout-sidebar 完全一致） ========== */
.layout-sidebar {
  width: 200px;
  background: var(--bg-card);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
}

.sidebar-nav {
  flex: 1;
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sidebar-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin: 0 8px;
  border-radius: 6px;
  text-decoration: none;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s;
}

.sidebar-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.sidebar-item.active {
  color: var(--color-primary);
  background: var(--bg-active);
  font-weight: 600;
}

.sidebar-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

/* ========== 分组二级菜单 ========== */
.sidebar-group {
  display: flex;
  flex-direction: column;
}

.sidebar-group-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin: 0 8px;
  border-radius: 6px;
  cursor: pointer;
  user-select: none;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s;
}

.sidebar-group-head:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.sidebar-group-head.active {
  color: var(--color-primary);
  background: var(--bg-active);
  font-weight: 600;
}

.group-caret {
  margin-left: auto;
  transition: transform 0.2s;
}

.group-caret.open {
  transform: rotate(180deg);
}

.sidebar-subnav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.sidebar-subnav .sidebar-item {
  padding-left: 32px;
}

/* ========== 右侧主内容区 ========== */
.layout-main {
  flex: 1;
  background: var(--bg-page);
  overflow-y: auto;
  padding: 24px;
}
</style>
