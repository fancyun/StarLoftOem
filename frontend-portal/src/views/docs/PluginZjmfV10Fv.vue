<template>
  <div class="markdown-body">
    <h1>智简魔方业务系统 v10 人脸核验插件（star_loft_fv_for_zjmf_v10）</h1>
    <p class="lead">
      平台标识：<code>star_loft_fv_for_zjmf_v10</code>。对接星楼网络人脸核验（FV）平台
      <strong>人脸核验</strong>能力，适用于<strong>智简魔方业务系统 v10</strong>，
      支持人脸识别、扫码承接与异步回调。
    </p>

    <h2>1. 功能特性</h2>
    <ul>
      <li>有源人脸识别（auth）：人脸 + 公安库真实身份比对</li>
      <li>无源人脸识别（self）：人脸与本人留底照片比对</li>
      <li>活体检测，防范照片 / 视频等攻击</li>
      <li>PC 展示二维码、手机扫码完成核身的承接流程</li>
      <li>按 v10 实名认证接口规范开发，支持自动扣费 / 免费次数</li>
      <li>认证结果异步回调 + 前台状态轮询，幂等与轮询上限保护</li>
      <li>安全的 HMAC-SHA256 签名认证</li>
    </ul>

    <h2>2. 安装步骤</h2>
    <h3>2.1 上传目录</h3>
    <p>将 <code>star_loft_fv_for_zjmf_v10</code> 目录上传到：</p>
    <pre><code>/public/plugins/certification/star_loft_fv_for_zjmf_v10/</code></pre>
    <h3>2.2 后台安装</h3>
    <ol>
      <li>登录 v10 后台 → <code>实名认证</code> → <code>接口管理</code></li>
      <li>找到 <code>StarLoft 人脸核验</code>，点击 <code>安装</code>，然后 <code>配置</code></li>
      <li>填写 API 地址、API Key、API Secret 及费用 / 免费次数 / 回跳地址</li>
    </ol>

    <h2>3. 使用流程</h2>
    <ol>
      <li>用户在 v10 会员中心进入人脸核验，选择 <code>StarLoft 人脸核验</code></li>
      <li>插件创建核身任务并返回跳转地址</li>
      <li>PC 进入 FV 承接页扫码，手机扫码完成核身；移动端直接跳转</li>
      <li>核身完成回跳，插件经回调/轮询同步结果</li>
    </ol>

    <h2>4. 对接接口</h2>
    <ul>
      <li><span class="method post">POST</span> <code>/api/fv/start</code> — 创建人脸核验订单（入参含 <code>fv_style</code>）</li>
      <li><span class="method post">POST</span> <code>/api/fv/result</code> — 查询核身结果</li>
    </ul>

    <h2>5. 结果回传</h2>
    <p>核身终态通过 <code>notify_url</code> 异步推送、<code>return_url</code> 同步回跳，字段含 <code>biz_no</code>、<code>status</code>、<code>result_code</code>、<code>cost</code>、<code>sign</code>，请校验签名（HMAC-SHA256）后落地。</p>
    <p>
      建议<strong>同时配置 notify_url 与 return_url</strong>：<code>notify_url</code> 负责服务端结果落地（最实时），
      <code>return_url</code> 负责用户侧页面回流。未配置 <code>notify_url</code> 时只能靠
      <code>/api/fv/result</code> 轮询，平台会按每笔核身的结果反查额度与最小间隔向上游同步，
      因此请勿按秒级高频轮询（建议 ≥ 5~10 秒并设轮询上限），高频轮询不会让结果更早出现。
    </p>

    <h2>6. 支持</h2>
    <ul>
      <li>插件版本 v1.0.0，作者 StarLoft，兼容智简魔方业务系统 v10</li>
    </ul>
  </div>
</template>

<script setup lang="ts"></script>