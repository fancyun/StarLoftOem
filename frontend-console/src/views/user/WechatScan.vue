<template>
  <div class="wechat-page">
    <div class="card">
      <h3 class="card-title">{{ title }}</h3>
      <p class="card-tip">{{ tip }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// PC 扫码授权完成后，微信内置浏览器跳转到本页只做结果展示：
// 授权与登录分离——手机侧只完成授权，登录态只在电脑端产生，本页不换取任何票据。
const route = useRoute()
const result = computed(() => String(route.query.r || ''))

const TEXTS: Record<string, { title: string; tip: string }> = {
  ok: { title: '授权成功', tip: '已完成微信授权，请回到电脑上继续操作。' },
  need_bind: { title: '授权成功', tip: '该微信尚未绑定平台账号，请在电脑上完成手机号绑定后即可登录。' },
  bound: { title: '绑定成功', tip: '已将该微信绑定到当前账号，请回到电脑继续操作。' },
  conflict: { title: '无法绑定', tip: '该微信已绑定其它账号，请先在该账号解绑后再试。' },
  used: { title: '二维码已被使用', tip: '请回到电脑刷新二维码后重新扫码。' },
  expired: { title: '二维码已过期', tip: '请回到电脑重新发起微信登录。' }
}

const current = computed(() => TEXTS[result.value] || { title: '授权完成', tip: '请回到电脑上继续操作。' })
const title = computed(() => current.value.title)
const tip = computed(() => current.value.tip)
</script>

<style scoped>
.wechat-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--bg-page);
}

.card {
  width: 100%;
  max-width: 460px;
  padding: 36px 32px;
  text-align: center;
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.card-tip {
  margin-top: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}
</style>