<template>
  <div class="markdown-body">
    <h1>插件使用教程</h1>
    <p class="lead">以下为 {{ productInfo.name }} 提供的下游系统对接插件，选择你的系统查看对应的安装与使用教程。</p>

    <template v-if="pluginGroups.length > 0">
      <template v-for="group in pluginGroups" :key="group.title">
        <h2>{{ group.title }}</h2>
        <table>
          <thead>
            <tr>
              <th>功能</th>
              <th>插件标识</th>
              <th>说明</th>
              <th>教程</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in group.items" :key="item.id">
              <td>{{ item.func }}</td>
              <td><code>{{ item.id }}</code></td>
              <td>{{ item.desc }}</td>
              <td><router-link :to="`/docs/${product}/plugin/${item.id}`">查看教程</router-link></td>
            </tr>
          </tbody>
        </table>
      </template>
    </template>

    <div v-else class="notice">该产品暂无下游系统对接插件。</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// 插件教程列表：/docs/{产品}/plugin
const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return ['fv', 'sms'].includes(p) ? p : 'fv'
})
const productNames: Record<string, string> = { fv: '人脸核验', sms: '短信服务' }
const productInfo = computed(() => ({ name: productNames[product.value] ?? '产品' }))

// 各产品插件（三级=适用系统，四级=插件类型）
const pluginGroups = computed(() => {
  if (product.value === 'fv') {
    return [
      {
        title: '智简魔方 · 财务版',
        items: [
          { id: 'star_loft_fv_for_zjmf_mfcw', func: '人脸核验（fv）', desc: '人脸核验（活体检测 + 人脸比对，支持扫码承接）' }
        ]
      },
      {
        title: '智简魔方业务系统 v10',
        items: [
          { id: 'star_loft_fv_for_zjmf_v10', func: '人脸核验（fv）', desc: '人脸核验（活体检测 + 人脸比对，支持扫码承接 / 异步回调）' }
        ]
      }
    ]
  }
  return []
})
</script>
