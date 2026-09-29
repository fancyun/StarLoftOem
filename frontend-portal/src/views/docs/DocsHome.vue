<template>
  <div class="markdown-body">
    <h1>{{ productInfo.name }}</h1>
    <p class="lead">{{ productInfo.lead }}</p>

    <h2>产品介绍</h2>
    <p>{{ productInfo.intro }}</p>

    <h2>文档目录</h2>

    <div class="doc-cards">
      <router-link :to="`/docs/${product}/api`" class="doc-card">
        <div class="doc-card-icon">API</div>
        <div class="doc-card-body">
          <h3>API 文档</h3>
          <p>面向开发者，介绍 {{ productInfo.name }} API 接口的鉴权方式、调用方法、请求与响应示例以及错误码。</p>
        </div>
      </router-link>

      <router-link v-if="productInfo.hasPlugin" :to="`/docs/${product}/plugin`" class="doc-card">
        <div class="doc-card-icon">插件</div>
        <div class="doc-card-body">
          <h3>插件使用教程</h3>
          <p>面向智简魔方等系统的管理员，介绍插件的安装、配置与使用方式。</p>
        </div>
      </router-link>

      <router-link :to="`/docs/${product}/pricing`" class="doc-card">
        <div class="doc-card-icon">定价</div>
        <div class="doc-card-body">
          <h3>定价</h3>
          <p>了解 {{ productInfo.name }} 的计费方式与单价。</p>
        </div>
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// 产品介绍文档：/docs/{产品}/docs
const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return ['fv', 'sms'].includes(p) ? p : 'fv'
})

const productInfos: Record<string, { name: string; lead: string; intro: string; hasPlugin: boolean }> = {
  fv: {
    name: '人脸核验',
    lead: '有源 / 无源人脸核验服务，面向实名认证与身份核验场景。',
    intro: '人脸核验（FV）提供有源（fv_auth：人脸 + 公安库真实身份比对）与无源（fv_self：人脸与本人留底比对）两种核身模式，支持 PC 扫码承接、移动端直接跳转，可对接智简魔方等下游系统。',
    hasPlugin: true
  },
  sms: {
    name: '短信服务',
    lead: '验证码 / 通知短信发送服务，支持签名、模板管理与发送统计。',
    intro: '短信服务（SMS）基于联麓（shlianlu）短信通道提供模板短信发送：签名与模板在线提交报备并经审核通过后使用（模板发送，验证码 / 通知场景），支持签名、模板管理、发送统计与回执查询；按短信资源包计费、余额兜底。',
    hasPlugin: false
  }
}

const productInfo = computed(() => productInfos[product.value] ?? productInfos.fv)
</script>

<style scoped>
.doc-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 20px;
  margin-top: 20px;
}

.doc-card {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 24px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  text-decoration: none;
  color: inherit;
  transition: all 0.15s;
}

.doc-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
}

.doc-card-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  border-radius: 10px;
  background: var(--color-primary-light);
  color: var(--color-primary);
  font-weight: 700;
  font-size: 13px;
  display: flex;
  align-items: center;
  justify-content: center;
  white-space: nowrap;
}

.doc-card-body h3 {
  margin: 0 0 6px;
  color: var(--text-primary);
  font-size: 17px;
}

.doc-card-body p {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
  line-height: 1.6;
}
</style>
