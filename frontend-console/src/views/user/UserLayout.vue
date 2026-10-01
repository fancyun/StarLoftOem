<template>
  <div class="user-layout">
    <!-- 顶部 Header（白底 + 下边框） -->
    <header class="layout-header">
      <div class="header-left">
        <!-- 窄屏汉堡按钮：唤起抽屉侧边栏 -->
        <button class="nav-toggle" type="button" aria-label="打开菜单" @click="toggleSidebar">
          <el-icon><Fold /></el-icon>
        </button>
        <router-link to="/dashboard" class="header-title">
          <img class="brand-logo" src="/logo.jpg" :alt="siteName" />
          <span class="header-title-text">{{ siteName }}控制台</span>
        </router-link>
      </div>
      <div class="header-right">
        <router-link to="/dashboard" class="user-name">
          <span>{{ userInfo?.phone || '用户' }}</span>
        </router-link>
      </div>
    </header>

    <div class="layout-body">
      <!-- 窄屏抽屉遮罩：点击关闭侧边栏 -->
      <div v-if="sidebarOpen" class="sidebar-mask" @click="closeSidebar"></div>
      <!-- 左侧侧边栏（顶部为一级分区切换，下方按分区渲染各自菜单，互不互通） -->
      <aside class="layout-sidebar" :class="{ open: sidebarOpen }">
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
                  v-for="s in sectionOptions"
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

          <div class="sidebar-footer">
            <button class="sidebar-item logout-btn" @click="handleLogout">
              <svg class="sidebar-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
                <polyline points="16 17 21 12 16 7"/>
                <line x1="21" y1="12" x2="9" y2="12"/>
              </svg>
              <span>退出登录</span>
            </button>
          </div>
        </nav>
      </aside>

      <!-- 右侧主内容区 -->
      <main class="layout-main">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowDown, Fold, Menu } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { siteName } from '@/utils/promotion'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const userInfo = computed(() => userStore.userInfo)

// ===== 窄屏（≤1024px）抽屉侧边栏 =====
const NARROW_SCREEN_MAX = 1024
const sidebarOpen = ref(false)

const toggleSidebar = () => {
  sidebarOpen.value = !sidebarOpen.value
}

const closeSidebar = () => {
  sidebarOpen.value = false
}

// 窗口放大回桌面宽度时收起抽屉，避免残留遮罩
const handleViewportResize = () => {
  if (window.innerWidth > NARROW_SCREEN_MAX && sidebarOpen.value) {
    closeSidebar()
  }
}

onMounted(() => window.addEventListener('resize', handleViewportResize))
onBeforeUnmount(() => window.removeEventListener('resize', handleViewportResize))

// ===== 一级分区 =====
const sectionOptions = [
  { key: 'user', label: '用户中心' },
  { key: 'fv', label: '人脸核验' },
  { key: 'sms', label: '短信服务' }
]

const section = computed(() => {
  const s = route.path.split('/')[1]
  return s === 'fv' || s === 'sms' ? s : 'user'
})

const currentSection = computed(() => sectionOptions.find((s) => s.key === section.value) || sectionOptions[0])

// 各分区侧边栏菜单（互不互通）
const sections: Record<string, { label: string; defaultPath: string; menu: any[] }> = {
  user: {
    label: '用户中心',
    defaultPath: '/dashboard',
    menu: [
      { type: 'link', to: '/dashboard', label: '控制台首页', icon: 'home' },
      { type: 'link', to: '/certification', label: '实名认证', icon: 'idcard' },
      { type: 'link', to: '/balance', label: '余额管理', icon: 'wallet' },
      { type: 'link', to: '/orders', label: '我的订单', icon: 'orders' },
      { type: 'link', to: '/api', label: 'API 管理', icon: 'code' },
      { type: 'link', to: '/promotion', label: '推广中心', icon: 'promotion' },
      { type: 'link', to: '/settings', label: '账户设置', icon: 'config' }
    ]
  },
  fv: {
    label: '人脸核验',
    defaultPath: '/fv/records',
    menu: [
      { type: 'link', to: '/fv/records', label: '认证记录', icon: 'orders' },
      { type: 'link', to: '/fv/invoke', label: '在线调用', icon: 'code' },
      { type: 'link', to: '/fv/packs', label: '资源包', icon: 'packs' }
    ]
  },
  sms: {
    label: '短信服务',
    defaultPath: '/sms/sign',
    menu: [
      { type: 'link', to: '/sms/sign', label: '签名', icon: 'sign' },
      { type: 'link', to: '/sms/template', label: '模板', icon: 'template' },
      { type: 'link', to: '/sms/send', label: '在线发送', icon: 'send' },
      { type: 'link', to: '/sms/packs', label: '资源包', icon: 'packs' },
      { type: 'link', to: '/sms/records', label: '发送记录', icon: 'records' },
      { type: 'link', to: '/sms/stats', label: '统计', icon: 'chart' }
    ]
  }
}

const currentMenu = computed(() => sections[section.value]?.menu || sections.user.menu)

// 菜单图标（内联 SVG 片段，受控内容）
const iconPaths: Record<string, string> = {
  home: '<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>',
  wallet: '<path d="M21 12V7H5a2 2 0 0 1 0-4h14v4"/><path d="M3 5v14a2 2 0 0 0 2 2h16v-5"/><path d="M18 12a2 2 0 0 0 0 4h4v-4z"/>',
  orders: '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
  code: '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>',
  packs: '<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>',
  sign: '<polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>',
  template: '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>',
  send: '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>',
  records: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
  chart: '<line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/>',
  promotion: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>',
  apply: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="12" y1="18" x2="12" y2="12"/><line x1="9" y1="15" x2="15" y2="15"/>',
  price: '<path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/><line x1="7" y1="7" x2="7.01" y2="7"/>',
  user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  finance: '<line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>',
  idcard: '<rect x="2" y="4" width="20" height="16" rx="2"/><circle cx="8" cy="10" r="2"/><path d="M4 20c1-3 2-4 4-4s3 1 4 4"/><line x1="14" y1="9" x2="20" y2="9"/><line x1="14" y1="13" x2="20" y2="13"/>',
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

const isLinkActive = (item: { to: string }) => {
  return route.path === item.to || route.path.startsWith(item.to + '/')
}

const isGroupActive = (item: { children: { to: string }[] }) => {
  return item.children.some((child) => isLinkActive(child))
}

// 分区/路径变化时自动展开包含当前页的折叠组，并收起窄屏抽屉
watch(
  [section, () => route.path],
  () => {
    closeSidebar()
    const menu = sections[section.value]?.menu || []
    for (const item of menu) {
      if (item.type === 'group' && item.children.some((child: { to: string }) => isLinkActive(child))) {
        openGroups.value.add(item.label)
      }
    }
  },
  { immediate: true }
)

const handleSectionSwitch = (key: string) => {
  const target = sections[key]
  closeSidebar()
  if (target && key !== section.value) {
    router.push(target.defaultPath)
  }
}

function handleLogout() {
  userStore.clearAuth()
  ElMessage.success('已退出登录')
  router.push('/login')
}
</script>

<style scoped>
.user-layout {
  --header-h: 60px;
  display: flex;
  flex-direction: column;
  height: 100vh;
  height: 100dvh; /* 移动端按可视区高度计算，避免浏览器地址栏遮挡 */
  overflow: hidden;
}

/* ========== 顶部 Header（腾讯云控制台风格：白底+下边框） ========== */
.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--header-h);
  padding: 0 24px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-right: auto;
  min-width: 0;
}

/* 窄屏汉堡按钮（桌面端隐藏） */
.nav-toggle {
  display: none;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  padding: 0;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md, 6px);
  background: var(--bg-card);
  color: var(--text-primary);
  font-size: 18px;
  cursor: pointer;
  flex-shrink: 0;
}

.nav-toggle:active {
  background: var(--bg-hover);
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
  min-width: 0;
}

.header-title-text {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-name {
  display: flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: var(--text-secondary);
  font-size: 14px;
  padding: 6px 14px;
  border-radius: 6px;
  transition: background 0.2s;
}

.user-name:hover {
  background: var(--bg-hover);
  color: var(--color-primary);
}

/* ========== 主体 ========== */
.layout-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* ========== 左侧白色侧边栏 ========== */
.layout-sidebar {
  width: 200px;
  background: var(--bg-card);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

/* 窄屏抽屉遮罩（桌面端不出现） */
.sidebar-mask {
  display: none;
  position: fixed;
  left: 0;
  right: 0;
  top: var(--header-h);
  bottom: 0;
  background: rgba(0, 0, 0, 0.45);
  z-index: 150;
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

.sidebar-footer {
  padding: 8px 0 16px;
  border-top: 1px solid var(--border-light);
  margin: 0 8px;
}

.logout-btn {
  width: 100%;
  background: none;
  border: none;
  cursor: pointer;
  font-family: inherit;
  color: var(--text-muted);
}

.logout-btn:hover {
  color: var(--color-danger);
  background: var(--color-danger-light);
}

/* ========== 右侧主内容区 ========== */
.layout-main {
  flex: 1;
  background: var(--bg-page);
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 24px;
}

/* ========== 响应式：平板 / 手机 ========== */

/* 平板及以下（≤1024px）：侧边栏改为左侧抽屉，汉堡按钮唤起 */
@media (max-width: 1024px) {
  .nav-toggle {
    display: inline-flex;
  }

  .layout-sidebar {
    position: fixed;
    left: 0;
    top: var(--header-h);
    bottom: 0;
    width: 240px;
    max-width: 78vw;
    transform: translateX(-100%);
    transition: transform 0.25s ease;
    box-shadow: var(--shadow-lg);
    z-index: 200;
  }

  .layout-sidebar.open {
    transform: translateX(0);
  }

  .sidebar-mask {
    display: block;
  }

  .layout-main {
    padding: 16px;
  }
}

/* 手机（≤768px）：进一步压缩留白与字号 */
@media (max-width: 768px) {
  .user-layout {
    --header-h: 56px;
  }

  .layout-header {
    padding: 0 12px;
  }

  .header-title {
    font-size: 15px;
    gap: 8px;
  }

  .header-title-text {
    max-width: 46vw;
  }

  .brand-logo {
    height: 24px;
  }

  .user-name {
    font-size: 13px;
    padding: 6px 8px;
    max-width: 32vw;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .layout-main {
    padding: 12px;
  }

  .sidebar-item,
  .sidebar-group-head {
    padding: 13px 16px;
  }
}

/* 小屏手机（≤480px）：抽屉加宽，便于点击 */
@media (max-width: 480px) {
  .layout-sidebar {
    width: 82vw;
    max-width: none;
  }
}
</style>