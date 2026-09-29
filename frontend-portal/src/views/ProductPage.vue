<template>
  <div class="product-page">
    <div v-if="product">
      <!-- 产品 Hero -->
      <section class="hero">
        <div class="container hero-inner">
          <div class="hero-icon" v-html="product.icon"></div>
          <h1 class="hero-title">{{ product.name }}</h1>
          <p class="hero-english">{{ product.english }}</p>
          <p class="hero-tagline">{{ product.tagline }}</p>
          <p class="hero-desc">{{ product.desc }}</p>
          <div class="hero-actions">
            <a
              v-if="product.status === 'available' && product.consolePath"
              class="btn-primary"
              :href="consoleUrl"
            >立即使用</a>
            <span v-else class="btn-primary btn-disabled">即将上线</span>
            <router-link class="btn-ghost" :to="`/docs/${product.key}/api/v1`">查看 API 文档</router-link>
          </div>
        </div>
      </section>

      <!-- 产品特性 -->
      <section class="section">
        <div class="container">
          <div class="section-head">
            <h2>产品特性</h2>
            <p>核心能力一览</p>
          </div>
          <div class="feature-grid">
            <div v-for="f in product.features" :key="f.title" class="feature-card">
              <h3>{{ f.title }}</h3>
              <p>{{ f.desc }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- 典型场景 -->
      <section class="section section-soft">
        <div class="container">
          <div class="section-head">
            <h2>典型场景</h2>
            <p>适用于多种业务形态</p>
          </div>
          <div class="scenario-list">
            <span v-for="s in product.scenarios" :key="s" class="scenario-item">{{ s }}</span>
          </div>
        </div>
      </section>

      <!-- CTA -->
      <section class="cta">
        <div class="container cta-inner">
          <h2>开始使用{{ product.name }}</h2>
          <p>注册账号即可体验，按需购买、灵活计费</p>
          <a class="btn-primary" :href="registerUrl">免费注册</a>
        </div>
      </section>
    </div>
    <div v-else class="container not-found">
      <h2>产品不存在</h2>
      <router-link to="/">返回首页</router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { resolveEntry } from '@/config/products'
import type { Product } from '@/config/products'
import { registerUrl, siteBase } from '@/utils/promotion'

const route = useRoute()

// 解析产品入口（按 meta.entryKey 定位一级产品：fv/sms）
const product = computed<Product | undefined>(() => resolveEntry(String(route.meta.entryKey)))

// 「立即使用」跳转地址：按当前域名归属取控制台站点（推广域名下指向其控制台），consolePath 为站内相对路径
const consoleUrl = computed(() =>
  product.value?.consolePath ? `${siteBase('console')}${product.value.consolePath}` : ''
)
</script>

<style scoped>
.product-page {
  background: var(--bg-page);
}

/* ========== Hero ========== */
.hero {
  background: linear-gradient(135deg, #15C5BA 0%, #1CD5C7 100%);
  color: #fff;
  padding: 72px 0;
  display: flex;
  align-items: center;
  min-height: 440px;
}

.hero-inner {
  max-width: 860px;
  margin: 0 auto;
  text-align: center;
}

.hero-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.15);
  color: #fff;
  margin-bottom: 20px;
}

.hero-icon :deep(svg) {
  width: 36px;
  height: 36px;
}

.hero-title {
  color: #fff;
  font-size: 36px;
  font-weight: 700;
  margin-bottom: 4px;
}

.hero-english {
  color: rgba(255, 255, 255, 0.8);
  font-size: 14px;
  letter-spacing: 1px;
  margin-bottom: 16px;
}

.hero-tagline {
  color: rgba(255, 255, 255, 0.95);
  font-size: 20px;
  font-weight: 600;
  margin-bottom: 16px;
}

.hero-desc {
  color: rgba(255, 255, 255, 0.85);
  font-size: 15px;
  line-height: 1.8;
  max-width: 680px;
  margin: 0 auto 36px;
}

.hero-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
}

.btn-primary {
  display: inline-block;
  padding: 12px 32px;
  border-radius: var(--radius-md);
  background: #fff;
  color: var(--color-primary);
  font-size: 16px;
  font-weight: 600;
  text-decoration: none;
  transition: all 0.15s;
}

.btn-primary:hover {
  background: var(--bg-soft);
  color: var(--color-primary-active);
}

.btn-disabled {
  cursor: not-allowed;
  opacity: 0.85;
}

.btn-ghost {
  display: inline-block;
  padding: 12px 32px;
  border-radius: var(--radius-md);
  border: 1px solid rgba(255, 255, 255, 0.7);
  color: #fff;
  font-size: 16px;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.15s;
}

.btn-ghost:hover {
  background: rgba(255, 255, 255, 0.15);
  color: #fff;
}

/* ========== 通用区块 ========== */
.section {
  padding: 72px 0;
}

.section-soft {
  background: var(--bg-soft);
}

.section-head {
  text-align: center;
  margin-bottom: 48px;
}

.section-head h2 {
  font-size: 28px;
  margin-bottom: 8px;
}

.section-head p {
  color: var(--text-muted);
  font-size: 15px;
}

/* ========== 特性卡片 ========== */
.feature-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 24px;
}

.feature-card {
  padding: 28px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xl);
  transition: all 0.15s;
}

.feature-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
}

.feature-card h3 {
  font-size: 17px;
  margin-bottom: 10px;
  color: var(--color-primary);
}

.feature-card p {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.7;
}

/* ========== 场景 ========== */
.scenario-list {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 16px;
}

.scenario-item {
  padding: 12px 28px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 999px;
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 500;
}

/* ========== CTA ========== */
.cta {
  background: var(--bg-card);
  border-top: 1px solid var(--border-color);
  padding: 64px 0;
}

.cta-inner {
  text-align: center;
}

.cta-inner h2 {
  font-size: 28px;
  margin-bottom: 8px;
}

.cta-inner p {
  color: var(--text-muted);
  font-size: 15px;
  margin-bottom: 32px;
}

/* ========== 兜底 ========== */
.not-found {
  padding: 120px 0;
  text-align: center;
}

.not-found h2 {
  margin-bottom: 16px;
}
</style>
