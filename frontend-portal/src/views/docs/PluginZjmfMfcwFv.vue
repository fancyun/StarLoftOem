<template>
  <div class="markdown-body">
    <h1>智简魔方·财务版 人脸核验插件（star_loft_fv_for_zjmf_mfcw）</h1>
    <p class="lead">
      平台标识：<code>star_loft_fv_for_zjmf_mfcw</code>。为智简魔方·财务版（3.7.6+）系统提供
      <strong>人脸核验</strong>能力（活体检测 + 人脸比对），对接星楼网络人脸核验（FV）平台。
    </p>

    <h2>1. 功能特性</h2>
    <ul>
      <li>有源人脸识别（auth）：人脸 + 公安库真实身份比对</li>
      <li>无源人脸识别（self）：人脸与本人留底照片比对</li>
      <li>活体检测，有效防范照片、视频等攻击手段</li>
      <li>PC 展示二维码、手机扫码完成核身的承接流程</li>
      <li>认证结果异步回调与状态轮询</li>
      <li>安全的 HMAC-SHA256 签名认证</li>
    </ul>

    <h2>2. 安装与流程</h2>
    <h3>2.1 上传插件</h3>
    <p>将 <code>star_loft_fv_for_zjmf_mfcw</code> 目录上传到：</p>
    <pre><code>/public/plugins/certification/star_loft_fv_for_zjmf_mfcw/</code></pre>
    <h3>2.2 使用流程</h3>
    <ol>
      <li>用户在会员中心发起人脸核验</li>
      <li>插件调用创建订单接口，平台返回核身跳转地址</li>
      <li>PC 端进入星楼网络 FV 承接页展示二维码，手机扫码完成核身；移动端直接跳转</li>
      <li>核身完成后回跳，插件轮询 / 回调更新状态</li>
    </ol>

    <h2>3. 对接接口</h2>
    <p>鉴权方式详见 <router-link :to="`/docs/${product}/api/v1`">API v1 文档</router-link>：</p>
    <ul>
      <li><span class="method post">POST</span> <code>/api/fv/start</code> — 创建人脸核验订单（入参含 <code>fv_style</code>：auth/self）</li>
      <li><span class="method post">POST</span> <code>/api/fv/result</code> — 查询核身结果</li>
    </ul>

    <h2>4. 结果回传</h2>
    <p>核身订单一账通过 <code>notify_url</code>（异步）与 <code>return_url</code>（同步）回流，字段含 <code>biz_no</code>、<code>status</code>、<code>result_code</code>、<code>cost</code>、<code>sign</code>，请校验签名后落地。</p>
    <p>
      建议<strong>同时配置 notify_url 与 return_url</strong>：<code>notify_url</code> 负责服务端结果落地，
      <code>return_url</code> 负责用户侧页面回流。未配置 <code>notify_url</code> 时只能靠
      <code>/api/fv/result</code> 轮询，平台会按每笔核身的结果反查额度与最小间隔向上游同步，
      请勿按秒级高频轮询（建议 ≥ 5~10 秒并设轮询上限），高频轮询不会让结果更早出现。
    </p>

    <h2>5. 环境要求</h2>
    <ul>
      <li>PHP &gt;= 7.0，curl / json 扩展，SSL 支持</li>
      <li>智简魔方·财务版 3.7.6+</li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'

// 人脸核验插件文档（部署于 /docs/fv/plugin/...）
const product = useRoute().path.split('/')[2]
</script>