<template>
  <div class="home">
    <!-- Hero 区 -->
    <section class="hero">
      <div class="container hero-inner">
        <h1 class="hero-title">一站式云服务平台</h1>
        <p class="hero-sub">
          为企业提供人脸核验、短信等安全可信的云产品能力，
          简单接入、稳定可靠、按需付费。
        </p>
        <div class="hero-actions">
          <a class="btn-primary" :href="registerUrl">免费注册</a>
          <router-link class="btn-ghost" to="/docs">了解产品服务</router-link>
        </div>
      </div>
    </section>

    <!-- 产品服务 -->
    <section class="section">
      <div class="container">
        <div class="section-head">
          <h2>产品与服务</h2>
          <p>覆盖实名、通信等基础云能力，持续扩充中</p>
        </div>
        <div class="product-grid">
          <router-link
            v-for="p in products"
            :key="p.key"
            :to="`/product/${p.key}`"
            class="product-card"
          >
            <span class="product-icon" v-html="p.icon"></span>
            <div class="product-info">
              <div class="product-name">
                {{ p.name }}
                <span v-if="p.status === 'coming-soon'" class="tag-coming">建设中</span>
              </div>
              <p class="product-desc">{{ p.desc }}</p>
              <span class="product-more">了解详情 →</span>
            </div>
          </router-link>
        </div>
      </div>
    </section>

    <!-- 平台优势 -->
    <section class="section section-soft">
      <div class="container">
        <div class="section-head">
          <h2>平台优势</h2>
          <p>稳定可靠的基础设施，让企业专注业务本身</p>
        </div>
        <div class="advantage-grid">
          <div v-for="a in advantages" :key="a.title" class="advantage-card">
            <div class="advantage-icon" v-html="a.icon"></div>
            <h3>{{ a.title }}</h3>
            <p>{{ a.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="cta">
      <div class="container cta-inner">
        <h2>立即开始使用{{ siteName }}</h2>
        <p>注册账号，开通产品服务，几分钟内完成对接</p>
        <a class="btn-primary" :href="registerUrl">注册</a>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { products } from '@/config/products'
import { registerUrl, siteName } from '@/utils/promotion'

const advantages = [
  {
    title: '安全可靠',
    desc: 'HMAC-SHA256 签名鉴权，敏感数据加密存储，认证结果防伪造',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>'
  },
  {
    title: '开放易用',
    desc: 'RESTful API 标准接口，配套 PHP 插件与完整文档，快速集成',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>'
  },
  {
    title: '灵活计费',
    desc: '资源包 + 余额组合计费，先扣资源包、余额兜底，成本可控',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="5" width="20" height="14" rx="2"/><line x1="2" y1="10" x2="22" y2="10"/></svg>'
  },
  {
    title: '高并发稳定',
    desc: 'Go 语言构建，Redis 缓存，支撑大规模并发认证请求',
    icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>'
  }
]
</script>

<style scoped>
.home {
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
  max-width: 900px;
  margin: 0 auto;
  text-align: center;
}

.hero-title {
  color: #fff;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: 1px;
  margin-bottom: 20px;
}

.hero-sub {
  color: rgba(255, 255, 255, 0.9);
  font-size: 18px;
  line-height: 1.7;
  margin-bottom: 40px;
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

/* ========== 产品卡片 ========== */
.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 24px;
}

.product-card {
  display: flex;
  align-items: flex-start;
  gap: 20px;
  padding: 28px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xl);
  text-decoration: none;
  transition: all 0.15s;
}

.product-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.product-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: var(--radius-lg);
  background: var(--color-primary-light);
  color: var(--color-primary);
  flex-shrink: 0;
}

.product-icon :deep(svg) {
  width: 28px;
  height: 28px;
}

.product-info {
  min-width: 0;
}

.product-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.product-desc {
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.7;
  margin-bottom: 12px;
}

.product-more {
  font-size: 14px;
  color: var(--color-primary);
  font-weight: 500;
}

.tag-coming {
  display: inline-block;
  padding: 1px 8px;
  border-radius: var(--radius-sm);
  background: var(--bg-soft);
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 500;
}

/* ========== 优势卡片 ========== */
.advantage-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 24px;
}

.advantage-card {
  padding: 32px 28px;
  background: var(--bg-card);
  border-radius: var(--radius-xl);
  text-align: center;
}

.advantage-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--radius-lg);
  background: var(--color-primary-light);
  color: var(--color-primary);
  margin-bottom: 16px;
}

.advantage-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

.advantage-card h3 {
  font-size: 17px;
  margin-bottom: 8px;
}

.advantage-card p {
  font-size: 14px;
  color: var(--text-muted);
  line-height: 1.7;
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

/* ========== 响应式：平板（≤1024px） ========== */
@media (max-width: 1024px) {
  .hero {
    padding: 56px 0;
    min-height: 360px;
  }

  .hero-title {
    font-size: 34px;
  }

  .section {
    padding: 56px 0;
  }
}

/* ========== 响应式：手机（≤768px） ========== */
@media (max-width: 768px) {
  .hero {
    padding: 44px 0;
    min-height: auto;
  }

  .hero-title {
    font-size: 28px;
    letter-spacing: 0.5px;
    margin-bottom: 14px;
  }

  .hero-sub {
    font-size: 15px;
    margin-bottom: 28px;
  }

  .hero-actions {
    flex-wrap: wrap;
    gap: 12px;
  }

  .btn-primary,
  .btn-ghost {
    padding: 11px 24px;
    font-size: 15px;
  }

  .section {
    padding: 40px 0;
  }

  .section-head {
    margin-bottom: 28px;
  }

  .section-head h2 {
    font-size: 22px;
  }

  .section-head p {
    font-size: 14px;
  }

  /* 产品卡片改单列铺满；优势卡片保持两列 */
  .product-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .advantage-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
  }

  .product-card {
    gap: 14px;
    padding: 20px;
  }

  .product-icon {
    width: 46px;
    height: 46px;
  }

  .product-icon :deep(svg) {
    width: 24px;
    height: 24px;
  }

  .product-name {
    font-size: 16px;
    margin-bottom: 6px;
  }

  .advantage-card {
    padding: 24px 20px;
  }

  .cta {
    padding: 40px 0;
  }

  .cta-inner h2 {
    font-size: 22px;
  }

  .cta-inner p {
    font-size: 14px;
    margin-bottom: 24px;
  }
}

/* ========== 响应式：小屏手机（≤480px） ========== */
@media (max-width: 480px) {
  .hero-title {
    font-size: 24px;
  }

  /* 优势卡片单列；主按钮整行铺满，便于点按 */
  .advantage-grid {
    grid-template-columns: 1fr;
  }

  .hero-actions {
    width: 100%;
  }

  .btn-primary,
  .btn-ghost {
    flex: 1 1 100%;
    text-align: center;
  }
}
</style>
