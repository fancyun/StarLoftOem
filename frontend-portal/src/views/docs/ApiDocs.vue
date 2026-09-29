<template>
  <div class="markdown-body">
    <h1>API 文档</h1>
    <p class="lead">星楼网络平台对外提供基于 API Key + HMAC-SHA256 签名鉴权的 RESTful 接口。</p>

    <h2>版本列表</h2>
    <table>
      <thead>
        <tr>
          <th>版本</th>
          <th>状态</th>
          <th>说明</th>
          <th>文档</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>v1</td>
          <td>当前版本</td>
          <td>{{ productInfo.name }}（{{ product }}）对外接口：发起认证（fv/auth、fv/self）/ 短信发送与模板管理（sms/*）等</td>
          <td><router-link :to="`/docs/${product}/api/v1`">查看文档</router-link></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// API 文档列表：/{产品}/api
const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return ['fv', 'sms'].includes(p) ? p : 'fv'
})
const productNames: Record<string, string> = { fv: '人脸核验', sms: '短信服务' }
const productInfo = computed(() => ({ name: productNames[product.value] ?? '产品' }))
</script>
