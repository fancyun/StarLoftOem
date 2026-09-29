<template>
  <div class="site-layout">
    <header class="site-header">
      <div class="container header-inner">
        <router-link to="/" class="brand">
          <img class="brand-logo" src="/logo.jpg" :alt="siteName" />
          <span class="brand-text">{{ siteName }}</span>
          <span v-if="siteSubName" class="brand-sub">{{ siteSubName }}</span>
        </router-link>

        <nav class="site-nav">
          <router-link to="/" class="nav-item" :class="{ active: $route.path === '/' }">首页</router-link>
          <div class="nav-dropdown" :class="{ open: menuOpen }">
            <span class="nav-item" @click="toggleMenu" :class="{ active: $route.path.startsWith('/product') }">
              产品
              <i class="dropdown-caret"></i>
            </span>
            <div class="dropdown-menu" :class="{ 'menu-open': menuOpen }">
              <router-link
                v-for="g in menuGroups"
                :key="g.key"
                :to="g.path"
                class="dropdown-item"
                @click="closeMenu"
              >{{ g.name }}</router-link>
            </div>
          </div>
          <router-link
            to="/docs"
            class="nav-item"
            :class="{ active: $route.path.startsWith('/docs') }"
          >文档中心</router-link>
        </nav>

        <div class="header-actions">
          <a class="action-btn action-btn-plain" :href="loginUrl">登录</a>
          <a class="action-btn" :href="registerUrl">注册</a>
        </div>

        <!-- 窄屏汉堡按钮：展开移动端导航面板（桌面端隐藏） -->
        <button
          class="nav-toggle"
          type="button"
          :aria-expanded="mobileOpen"
          aria-label="导航菜单"
          @click="toggleMobileNav"
        >
          <span class="nav-toggle-box" :class="{ open: mobileOpen }">
            <i></i><i></i><i></i>
          </span>
        </button>
      </div>

      <!-- 移动端导航面板（≤768px 显示，点击导航项后自动收起） -->
      <div v-show="mobileOpen" class="mobile-panel">
        <nav class="container mobile-nav">
          <router-link to="/" class="mobile-item" @click="closeMobileNav">首页</router-link>

          <div class="mobile-group-title">产品</div>
          <router-link
            v-for="g in menuGroups"
            :key="g.key"
            :to="g.path"
            class="mobile-item mobile-item-sub"
            @click="closeMobileNav"
          >{{ g.name }}</router-link>

          <router-link to="/docs" class="mobile-item" @click="closeMobileNav">文档中心</router-link>

          <div class="mobile-actions">
            <a class="action-btn action-btn-plain" :href="loginUrl">登录</a>
            <a class="action-btn" :href="registerUrl">注册</a>
          </div>
        </nav>
      </div>
    </header>

    <main class="site-main">
      <router-view />
    </main>

    <footer class="site-footer">
      <div class="container footer-inner">
        <div class="footer-cols">
          <div class="footer-col footer-brand-col">
            <div class="footer-brand">
              <img class="brand-logo" src="/logo.jpg" :alt="siteName" />
              <span>{{ siteName }}</span>
            </div>
            <p class="footer-desc">一站式云服务平台，为企业提供安全、稳定、易用的云产品能力。</p>
          </div>
          <div class="footer-col">
            <div class="footer-title">产品服务</div>
            <router-link
              v-for="g in menuGroups"
              :key="g.key"
              :to="g.path"
              class="footer-link"
            >{{ g.name }}</router-link>
          </div>
          <div class="footer-col">
            <div class="footer-title">开发文档</div>
            <router-link to="/docs" class="footer-link">文档总览</router-link>
            <router-link to="/docs/fv/api/v1" class="footer-link">API v1 文档</router-link>
            <router-link to="/docs/fv/plugin" class="footer-link">插件教程</router-link>
          </div>
          <div v-if="hasContact" class="footer-col">
            <div class="footer-title">联系我们</div>
            <a v-if="contact.email" class="footer-link" :href="`mailto:${contact.email}`">{{ contact.email }}</a>
            <a v-if="contact.phone" class="footer-link" :href="`tel:${contact.phone}`">{{ contact.phone }}</a>
            <span v-if="contact.wechat" class="footer-link">微信：{{ contact.wechat }}</span>
            <span v-if="contact.qq" class="footer-link">QQ：{{ contact.qq }}</span>
            <span v-if="contact.hours" class="footer-link">服务时间：{{ contact.hours }}</span>
          </div>

          <div class="footer-col">
            <div class="footer-title">公司信息</div>
            <span class="footer-link">上海星楼网络科技有限公司</span>
            <span class="footer-link">上海市崇明区东平镇东冉路547号</span>
          </div>

          <div class="footer-col">
            <div class="footer-title">资源入口</div>
            <a class="footer-link" :href="siteBase('console')">云控制台</a>
            <a class="footer-link" :href="`${siteBase('console')}/login`">控制台登录</a>
            <a class="footer-link" :href="registerUrl">账号注册</a>
          </div>
        </div>
        <div class="footer-bottom">
          <span>{{ copyrightText }}</span>
          <a class="icp-link" href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">沪ICP备2026043262号</a>
          <router-link to="/terms" class="icp-link">用户协议</router-link>
          <router-link to="/privacy" class="icp-link">隐私政策</router-link>
          <router-link to="/auth-authorization" class="icp-link">实名认证授权</router-link>
          <router-link to="/sms-rules" class="icp-link">短信服务条款</router-link>
          <router-link to="/sla" class="icp-link">服务等级协议</router-link>
          <router-link to="/billing" class="icp-link">费用与退款</router-link>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { products } from '@/config/products'
import type { Product } from '@/config/products'
import { loginUrl, registerUrl, siteBase, siteName, siteSubName } from '@/utils/promotion'

const route = useRoute()

// 版权年份动态取当前年
const currentYear = new Date().getFullYear()

// 版权文案随品牌名变化（命中推广品牌时隐藏平台副品牌）
const copyrightText = computed<string>(
  () =>
    `© ${currentYear} ${siteName.value}${siteSubName.value ? ' ' + siteSubName.value : ''} · 保留所有权利`
)

// 由 products.ts 驱动的产品菜单
interface MenuGroup {
  key: string
  name: string
  path: string
}
const menuGroups = ref<MenuGroup[]>(
  products.map((p: Product) => ({
    key: p.key,
    name: p.name,
    path: `/product/${p.key}`
  }))
)

// 产品下拉：桌面端 hover 展开，移动端触屏点击展开（menuOpen）
const menuOpen = ref(false)

const toggleMenu = () => {
  menuOpen.value = !menuOpen.value
}

const closeMenu = () => {
  menuOpen.value = false
}

// 移动端导航面板（≤768px 汉堡展开）
const mobileOpen = ref(false)

const toggleMobileNav = () => {
  mobileOpen.value = !mobileOpen.value
  closeMenu()
}

const closeMobileNav = () => {
  mobileOpen.value = false
}

// 点击下拉区域之外时收起菜单
const onClickOutside = (e: MouseEvent) => {
  if (!(e.target as HTMLElement).closest('.nav-dropdown')) {
    closeMenu()
  }
}

// 视口放大回桌面宽度时收起移动端面板，避免残留遮挡
const handleViewportResize = () => {
  if (window.innerWidth > 768 && mobileOpen.value) {
    closeMobileNav()
  }
}

// 客服联系方式：门户为纯静态站点，页脚联系方式由后端公开配置接口实时下发
// （后台「系统设置 → 客服联系方式」维护，改完无需重启后端；未配置的项由后端按内置默认值兜底）
interface ContactInfo {
  email: string
  phone: string
  wechat: string
  qq: string
  hours: string
}
const contact = ref<ContactInfo>({ email: '', phone: '', wechat: '', qq: '', hours: '' })
const hasContact = computed(
  () => !!(contact.value.email || contact.value.phone || contact.value.wechat || contact.value.qq || contact.value.hours)
)

const loadContact = async () => {
  try {
    const resp = await fetch(`${siteBase('console')}/console/config`)
    const body = await resp.json()
    const data = body?.data?.contact
    if (data) {
      contact.value = {
        email: data.email || '',
        phone: data.phone || '',
        wechat: data.wechat || '',
        qq: data.qq || '',
        hours: data.hours || ''
      }
    }
  } catch {
    // 拉取失败时保持不展示，避免与后台配置不一致
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
  window.addEventListener('resize', handleViewportResize)
  loadContact()
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onClickOutside)
  window.removeEventListener('resize', handleViewportResize)
})

// 路由变化时收起产品下拉与移动端面板
watch(() => route.path, () => {
  closeMenu()
  closeMobileNav()
})
</script>

<style scoped>
.site-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}

/* ========== 顶部导航（白底 + 下边框） ========== */
.site-header {
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
}

.header-inner {
  display: flex;
  align-items: center;
  height: 60px;
  gap: 32px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-right: auto;
  text-decoration: none;
  color: var(--text-primary);
}

.brand-text {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

.brand-sub {
  font-size: 12px;
  color: var(--text-muted);
  letter-spacing: 0.5px;
  margin-top: 8px;
}

.site-nav {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
}

.nav-item {
  padding: 8px 14px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.15s;
}

.nav-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.nav-item.active {
  color: var(--color-primary);
  background: var(--bg-active);
  font-weight: 600;
}

/* 产品下拉菜单（hover 展开） */
.nav-dropdown {
  position: relative;
}

.nav-dropdown .nav-item {
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.dropdown-caret {
  width: 0;
  height: 0;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-top: 5px solid currentColor;
  transition: transform 0.15s;
}

.nav-dropdown:hover .dropdown-caret,
.nav-dropdown.open .dropdown-caret {
  transform: rotate(180deg);
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  left: 0;
  min-width: 160px;
  max-height: 70vh;   /* 限制最大高度，超出后内部滚动，避免产品增多时下拉溢出屏幕 */
  overflow-y: auto;
  padding: 6px;
  background: var(--bg-panel, #fff);
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  opacity: 0;
  visibility: hidden;
  transform: translateY(6px);
  transition: opacity 0.15s, transform 0.15s, visibility 0.15s;
  z-index: 100;
}

/* hover（桌面端）与点击展开（移动端触屏）两种方式均可显示菜单 */
.nav-dropdown:hover .dropdown-menu,
.nav-dropdown:focus-within .dropdown-menu,
.nav-dropdown .dropdown-menu.menu-open {
  opacity: 1;
  visibility: visible;
  transform: translateY(0);
}

.dropdown-item {
  display: block;
  padding: 9px 14px;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  font-size: 14px;
  text-decoration: none;
  transition: all 0.15s;
}

.dropdown-item:hover {
  color: var(--color-primary);
  background: var(--bg-hover);
}

.dropdown-item.router-link-active {
  color: var(--color-primary);
  background: var(--bg-active);
  font-weight: 600;
}

/* 二级子产品分组：分组标题 + 子项 */
.dropdown-group {
  padding: 2px 0 4px;
}
.dropdown-group + .dropdown-group {
  border-top: 1px solid var(--border-light, #eef0f3);
  margin-top: 4px;
  padding-top: 6px;
}
.dropdown-group-title {
  display: block;
  padding: 7px 14px 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-decoration: none;
}
.dropdown-group-title:hover {
  color: var(--color-primary);
}
.dropdown-item.sub {
  padding-left: 24px;
  font-size: 13px;
}
.mini-tag {
  display: inline-block;
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--bg-soft);
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 400;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-left: auto;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 8px 18px;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  background: var(--color-primary);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: background 0.15s;
}

.action-btn:hover {
  background: var(--color-primary-hover);
  color: #fff;
}

/* 登录：与注册按钮同尺寸，采用描边风格突出主次 */
.action-btn.action-btn-plain {
  background: transparent;
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.action-btn.action-btn-plain:hover {
  background: var(--bg-active);
  color: var(--color-primary);
}

/* ========== 窄屏汉堡按钮（桌面端隐藏） ========== */
.nav-toggle {
  display: none;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--bg-card);
  cursor: pointer;
  flex-shrink: 0;
  -webkit-tap-highlight-color: transparent;
}

.nav-toggle-box {
  position: relative;
  display: block;
  width: 18px;
  height: 14px;
}

.nav-toggle-box i {
  position: absolute;
  left: 0;
  width: 100%;
  height: 2px;
  border-radius: 2px;
  background: var(--text-primary);
  transition: transform 0.2s ease, opacity 0.2s ease, top 0.2s ease;
}

.nav-toggle-box i:nth-child(1) { top: 0; }
.nav-toggle-box i:nth-child(2) { top: 6px; }
.nav-toggle-box i:nth-child(3) { top: 12px; }

/* 展开时三横线变叉号 */
.nav-toggle-box.open i:nth-child(1) { top: 6px; transform: rotate(45deg); }
.nav-toggle-box.open i:nth-child(2) { opacity: 0; }
.nav-toggle-box.open i:nth-child(3) { top: 6px; transform: rotate(-45deg); }

/* ========== 移动端导航面板（桌面端不显示，由 v-show 控制开关） ========== */
.mobile-panel {
  display: none;
  position: absolute;
  left: 0;
  right: 0;
  top: 100%;
  background: var(--bg-card);
  border-top: 1px solid var(--border-color);
  box-shadow: var(--shadow-lg);
  max-height: calc(100vh - 56px);
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  z-index: 99;
}

.mobile-nav {
  display: flex;
  flex-direction: column;
  padding-top: 8px;
  padding-bottom: 16px;
}

.mobile-item {
  display: block;
  padding: 12px 4px;
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 500;
  text-decoration: none;
  border-bottom: 1px solid var(--border-light);
}

.mobile-item.router-link-active {
  color: var(--color-primary);
  font-weight: 600;
}

.mobile-item-sub {
  padding-left: 16px;
  font-size: 14px;
}

.mobile-group-title {
  padding: 14px 4px 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.mobile-actions {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.mobile-actions .action-btn {
  flex: 1;
}

/* ========== 主体 ========== */
.site-main {
  flex: 1;
}

/* ========== 页脚 ========== */
.site-footer {
  background: var(--bg-card);
  border-top: 1px solid var(--border-color);
  padding: 40px 0 0;
}

.footer-inner {
  padding-bottom: 24px;
}

.footer-cols {
  display: grid;
  /* 星楼网络单独占满左侧一列，其余信息在右侧（左侧一列 + 右侧三列两行） */
  grid-template-columns: 260px repeat(3, 1fr);
  grid-template-rows: auto auto;
  gap: 32px;
}

/* 品牌列：左侧单独竖排，跨两行占满高度 */
.footer-brand-col {
  grid-row: 1 / span 2;
  grid-column: 1;
}

.footer-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.footer-desc {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.7;
}

.footer-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 12px;
}

.footer-link {
  display: block;
  padding: 4px 0;
  font-size: 13px;
  color: var(--text-secondary);
  text-decoration: none;
  transition: color 0.15s;
}

.footer-link:hover {
  color: var(--color-primary);
}

.footer-subgroup {
  margin-bottom: 10px;
}
.footer-link-title {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 13px;
}
.footer-sub-link {
  padding-left: 12px;
  font-size: 12px;
  color: var(--text-secondary);
}

.footer-bottom {
  margin-top: 32px;
  padding-top: 16px;
  border-top: 1px solid var(--border-light);
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: 4px 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.icp-link {
  color: var(--text-muted);
  text-decoration: none;
  transition: color 0.15s;
}

.icp-link:hover {
  color: var(--color-primary);
}

/* ========== 响应式：平板（≤900px） ========== */
@media (max-width: 900px) {
  .footer-cols {
    grid-template-columns: 1fr 1fr;
    gap: 24px;
  }
  /* 品牌列恢复为普通一列，不再跨两行 */
  .footer-brand-col {
    grid-row: auto;
    grid-column: 1 / -1;
  }
  .brand-sub {
    display: none;
  }
  .header-inner {
    gap: 16px;
  }
  .header-actions {
    gap: 8px;
  }
  .nav-item {
    padding: 8px 10px;
  }
  .site-footer {
    padding-top: 32px;
  }
}

/* ========== 响应式：手机（≤768px） ========== */
@media (max-width: 768px) {
  .container {
    padding: 0 16px;
  }

  .header-inner {
    height: 56px;
    gap: 8px;
  }

  .brand-text {
    font-size: 16px;
  }

  /* 桌面导航与操作按钮收起，统一由汉堡菜单承载 */
  .site-nav,
  .header-actions {
    display: none;
  }

  .nav-toggle {
    display: inline-flex;
  }

  .mobile-panel {
    display: block;
  }

  .site-footer {
    padding-top: 28px;
  }

  .footer-cols {
    gap: 20px 16px;
  }

  .footer-bottom {
    margin-top: 24px;
    gap: 4px 10px;
  }
}

/* ========== 响应式：小屏手机（≤480px） ========== */
@media (max-width: 480px) {
  .brand-text {
    font-size: 15px;
  }

  .footer-cols {
    grid-template-columns: 1fr;
    gap: 20px;
  }

  .footer-desc {
    font-size: 12px;
  }
}
</style>
