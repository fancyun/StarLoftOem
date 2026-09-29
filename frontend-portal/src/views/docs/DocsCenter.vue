<template>
  <div class="docs-center">
    <header class="center-header">
      <div class="center-header-inner">
        <router-link to="/" class="brand">
          <img class="brand-logo" src="/logo.jpg" alt="星楼网络" />
          <span class="brand-text">星楼网络 · 文档中心</span>
        </router-link>
        <router-link to="/" class="home-link">返回首页</router-link>
      </div>
    </header>

    <main class="center-main">
      <h1 class="center-title">选择产品，查看对应文档</h1>
      <p class="center-sub">按产品查阅产品介绍、API、插件与定价；一级为产品，进入后二级为 API / 插件等。</p>

      <div class="product-grid">
        <router-link
          v-for="p in availableProducts"
          :key="p.key"
          :to="`/docs/${p.key}/docs`"
          class="product-card"
        >
          <span class="product-icon" v-html="p.icon" />
          <div class="product-title">{{ p.name }}</div>
          <div class="product-english">{{ p.english }}</div>
          <div class="product-desc">{{ p.desc }}</div>
          <span class="product-link">进入文档 →</span>
        </router-link>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { products } from '@/config/products'

// 文档中心一级入口：仅展示已上线的产品
const availableProducts = computed(() =>
  products.filter((p) => p.status === 'available')
)
</script>

<style scoped>
.docs-center {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.center-header {
  height: 56px;
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  position: sticky;
  top: 0;
  z-index: 100;
}

.center-header-inner {
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

.brand-logo {
  display: inline-block;
  height: 28px;
  width: auto;
  object-fit: contain;
}

.home-link {
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 13px;
}

.home-link:hover {
  color: var(--color-primary);
}

.center-main {
  flex: 1;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 56px 24px;
  text-align: center;
}

.center-title {
  font-size: 26px;
  margin: 0 0 8px;
  color: var(--text-primary);
}

.center-sub {
  margin: 0 auto 40px;
  color: var(--text-muted);
  font-size: 15px;
  max-width: 560px;
}

.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
  text-align: left;
}

.product-card {
  display: block;
  padding: 28px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-card);
  text-decoration: none;
  color: var(--text-primary);
  transition: all 0.18s;
}

.product-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.product-icon {
  display: inline-flex;
  width: 44px;
  height: 44px;
  border-radius: 10px;
  align-items: center;
  justify-content: center;
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.product-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.product-title {
  font-size: 18px;
  font-weight: 700;
  margin-top: 16px;
  color: var(--text-primary);
}

.product-english {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 2px;
}

.product-desc {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin-top: 12px;
}

.product-link {
  display: inline-block;
  margin-top: 16px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-primary);
}
</style>