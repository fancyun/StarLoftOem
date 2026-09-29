<template>
  <div class="docs-layout">
    <header class="docs-header">
      <div class="docs-header-inner">
        <router-link :to="`/docs/${product}/docs`" class="brand">
          <img class="brand-logo" src="/logo.jpg" alt="星楼网络" />
          <span class="brand-text">星楼网络 · {{ productLabel }}文档</span>
        </router-link>
      </div>
    </header>

    <div class="docs-body">
      <aside class="docs-sidebar">
        <router-link to="/docs" class="back-home">
          <span class="back-home-icon">‹</span>
          返回文档中心
        </router-link>
        <nav class="docs-nav">
          <div v-for="group in navGroups" :key="group.title" class="nav-group">
            <div class="nav-group-title">{{ group.title }}</div>

            <!-- 一级分组下的二级导航：产品 / API / 插件 / 计费 -->
            <template v-if="group.subGroups">
              <div v-for="sub in group.subGroups" :key="sub.label" class="nav-subgroup">
                <div class="nav-subgroup-title">{{ sub.label }}</div>
                <router-link
                  v-for="item in sub.items"
                  :key="item.path"
                  :to="item.path"
                  class="nav-item sub"
                  :class="{ active: $route.path === item.path }"
                >
                  {{ item.label }}
                </router-link>
              </div>
            </template>
          </div>
        </nav>
      </aside>

      <main class="docs-main">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// 产品与文档分层：/docs/{产品}/{内容}/...（docs 产品介绍、api、api/v1、plugin、pricing）
const products: Record<string, { name: string }> = {
  fv: { name: '人脸核验' },
  sms: { name: '短信服务' }
}

const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return p in products ? p : 'fv'
})
const productLabel = computed(() => products[product.value]?.name ?? '')

// 各产品插件（二级「插件」下的条目，按适用系统细分）
const pluginNavItems = computed(() => {
  if (product.value === 'fv') {
    return [
      { label: '智简魔方 · 财务版', path: '/docs/fv/plugin/star_loft_fv_for_zjmf_mfcw' },
      { label: '智简魔方业务系统 v10', path: '/docs/fv/plugin/star_loft_fv_for_zjmf_v10' }
    ]
  }
  return []
})

const navGroups = computed(() => {
  const p = product.value
  // 二级导航：开始 / API 文档 / 计费 / 插件
  const children: any[] = [
    { label: '开始', items: [{ label: '产品介绍', path: `/docs/${p}/docs` }] },
    { label: 'API 文档', items: [{ label: '版本列表', path: `/docs/${p}/api` }, { label: 'API v1', path: `/docs/${p}/api/v1` }] },
    { label: '计费', items: [{ label: '定价', path: `/docs/${p}/pricing` }] }
  ]
  if (pluginNavItems.value.length > 0) {
    children.push({ label: '插件', items: pluginNavItems.value })
  }
  // 一级：以当前产品为分组标题，其下为 API/插件/计费等二级导航
  return [{ title: productLabel.value, subGroups: children }]
})
</script>

<style scoped>
.docs-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.docs-header {
  height: 56px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: 100;
}

.docs-header-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-right: auto;
  color: var(--text-primary);
  text-decoration: none;
  font-weight: 700;
  font-size: 16px;
}

.brand:hover {
  color: var(--color-primary);
}

.docs-body {
  flex: 1;
  display: flex;
  width: 100%;
}

.docs-sidebar {
  width: 240px;
  flex-shrink: 0;
  border-right: 1px solid var(--border-color);
  background: var(--bg-card);
  padding: 20px 0;
}

.back-home {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 12px 16px;
  padding: 9px 12px;
  border-radius: 6px;
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 14px;
  font-weight: 600;
  transition: all 0.15s;
}

.back-home:hover {
  color: var(--color-primary);
  background: var(--bg-hover);
}

.back-home-icon {
  font-size: 18px;
  line-height: 1;
}

.docs-nav {
  position: sticky;
  top: 80px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 0 12px;
}

.nav-group-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin: 0 12px 8px;
}

.nav-item {
  display: block;
  padding: 8px 12px;
  border-radius: 6px;
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 14px;
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

/* 二级菜单（子分组）样式 */
.nav-subgroup {
  margin: 2px 0 12px;
}

.nav-subgroup-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 12px 4px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border-color);
}

.nav-item.sub {
  padding-left: 24px;
  font-size: 13px;
}

.docs-main {
  flex: 1;
  padding: 40px 48px;
  min-width: 0;
  max-width: 1100px;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .docs-sidebar {
    display: none;
  }
  .docs-main {
    padding: 24px 20px;
  }
}
</style>

<style>
/* 文档正文通用样式（由 DocsLayout 统一提供，供子页面复用） */
.markdown-body {
  color: var(--text-secondary);
  line-height: 1.75;
  font-size: 15px;
  max-width: 860px;
}

.markdown-body h1 {
  font-size: 28px;
  margin: 0 0 8px;
  color: var(--text-primary);
}

.markdown-body .lead {
  font-size: 16px;
  color: var(--text-muted);
  margin: 0 0 24px;
}

.markdown-body h2 {
  font-size: 22px;
  margin: 36px 0 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
  color: var(--text-primary);
}

.markdown-body h3 {
  font-size: 18px;
  margin: 24px 0 12px;
  color: var(--text-primary);
}

.markdown-body p {
  margin: 12px 0;
}

.markdown-body .notice {
  margin: 16px 0;
  padding: 12px 16px;
  background: var(--color-primary-light);
  border-left: 4px solid var(--color-primary);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  font-size: 14px;
}

.markdown-body ul,
.markdown-body ol {
  margin: 12px 0;
  padding-left: 24px;
}

.markdown-body li {
  margin: 6px 0;
}

.markdown-body code {
  background: var(--bg-soft);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', 'Courier New', monospace;
  font-size: 13px;
  color: #1f2937;
}

.markdown-body pre {
  background: #0f172a;
  color: #e2e8f0;
  padding: 16px 20px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 16px 0;
  font-size: 13px;
  line-height: 1.6;
}

.markdown-body pre code {
  background: transparent;
  color: inherit;
  padding: 0;
  font-size: 13px;
}

.markdown-body table {
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0;
}

.markdown-body th,
.markdown-body td {
  border: 1px solid var(--border-color);
  padding: 10px 12px;
  text-align: left;
  vertical-align: top;
}

.markdown-body th {
  background: var(--bg-soft);
  font-weight: 600;
  color: var(--text-primary);
}

.markdown-body .method {
  display: inline-block;
  font-weight: 700;
  font-family: 'JetBrains Mono', 'Courier New', monospace;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  margin-right: 8px;
}

.markdown-body .method.post {
  background: var(--color-success-light);
  color: var(--color-success);
}

.markdown-body .method.get {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.markdown-body .method.delete {
  background: var(--color-danger-light);
  color: var(--color-danger);
}
</style>
