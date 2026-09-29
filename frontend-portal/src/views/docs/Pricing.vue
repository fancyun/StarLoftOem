<template>
  <div class="markdown-body">
    <h1>定价</h1>
    <p class="lead">{{ productInfo.name }}的计费方式与说明。</p>

    <h2>计费方式</h2>
    <table>
      <thead>
        <tr>
          <th>计费项</th>
          <th>方式</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="row.item">
          <td>{{ row.item }}</td>
          <td>{{ row.mode }}</td>
          <td>{{ row.desc }}</td>
        </tr>
      </tbody>
    </table>

    <h2>费用结算</h2>
    <p>使用前请先在星楼网络控制台完成实名认证并确保账户余额充足。支持资源包（先扣资源包、余额兜底）与余额直接扣费。</p>
    <p>各产品实际单价以控制台展示为准。</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// 定价文档：/docs/{产品}/pricing
const route = useRoute()
const product = computed(() => {
  const p = route.path.split('/')[2]
  return ['fv', 'sms'].includes(p) ? p : 'fv'
})

const productInfos: Record<string, { name: string; rows: Array<{ item: string; mode: string; desc: string }> }> = {
  fv: {
    name: '人脸核验',
    rows: [
      { item: '有源人脸核验（fv_auth）', mode: '按次计费', desc: '人脸 + 公安库真实身份比对，按次扣费，可用独立资源包' },
      { item: '无源人脸识别（fv_self）', mode: '按次计费', desc: '人脸与本人留底照片比对，按次扣费，可用独立资源包' }
    ]
  },
  sms: {
    name: '短信服务',
    rows: [
      { item: '短信发送', mode: '资源包计费', desc: '按短信资源包扣除条数，余额兜底；仅支持模板发送（验证码 / 通知场景）' },
      { item: '签名 / 模板管理', mode: '免费', desc: '在线提交签名与模板报备（联麓审核），提交与审核不收费' }
    ]
  }
}

const productInfo = computed(() => productInfos[product.value] ?? productInfos.fv)
const rows = computed(() => productInfo.value.rows)
</script>
