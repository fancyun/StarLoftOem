<template>
  <div class="fv-verify">
    <!-- 订单已结束：结果界面（不再展示二维码 / 上游核身页） -->
    <div v-if="terminal" class="result-page">
      <div class="result-icon" :class="success ? 'ok' : 'fail'">{{ success ? '✓' : '!' }}</div>
      <h1>{{ resultTitle }}</h1>
      <p>{{ resultText }}</p>
      <p v-if="redirectUrl" class="redirect-tip">{{ redirectLeft }} 秒后自动返回商户页面…</p>
      <a v-if="redirectUrl" class="btn" :href="redirectUrl">立即返回</a>
      <div v-if="bizNo" class="biz-no">业务单号：{{ bizNo }}</div>
    </div>

    <!-- 参数缺失 / 错误态 -->
    <div v-else-if="errorMsg" class="err-page">
      <div class="err-icon">!</div>
      <h1>无法发起人脸核验</h1>
      <p>{{ errorMsg }}</p>
      <a class="btn" :href="siteBase('console')">前往{{ brandName }}控制台</a>
    </div>

    <!-- 移动端：代理上游实名界面（内嵌 iframe，保留 service 页面壳） -->
    <div v-else-if="isMobile" class="mobile-proxy">
      <div class="proxy-bar">
        <span>正在通过{{ brandName }}完成人脸核验</span>
        <a class="proxy-open" :href="upstreamUrl" target="_blank">新窗口打开</a>
      </div>
      <!-- allow 授予 iframe（跨域的上游核身页）摄像头/麦克风权限：
           缺少该 Permissions Policy 时手机浏览器会直接拒绝 getUserMedia，上游报错 6100 -->
      <iframe v-if="upstreamUrl" class="proxy-frame" :src="upstreamUrl" allow="camera; microphone"></iframe>
      <div class="proxy-footer">
        <button class="btn" @click="finish">我已完成核验</button>
      </div>
    </div>

    <!-- PC 端：二维码承接页 -->
    <div v-else class="pc-page">
      <div class="pc-card">
        <h1>人脸核验</h1>
        <p class="mode-tip">{{ modeLabel }}</p>
        <div class="qr-wrap" v-if="qrSvg">
          <div class="qr-box" v-html="qrSvg"></div>
        </div>
        <div v-else class="qr-alt">
          <p>二维码生成失败，请点击下方链接完成核身</p>
          <a class="link" :href="upstreamUrl" target="_blank">{{ upstreamUrl }}</a>
        </div>
        <p class="scan-tip">请使用手机微信 / 浏览器扫码，完成人脸核验</p>

        <div class="countdown">
          核身链接有效期剩余
          <span class="count">{{ countdown }}</span>
          秒
        </div>

        <div class="actions">
          <button class="btn" @click="finish">我已完成核身</button>
          <button class="btn ghost" @click="copyLink">复制认证链接</button>
        </div>
        <p v-if="copied" class="action-tip">认证链接已复制，可在手机浏览器粘贴打开完成核身</p>
        <p v-if="finishedTip" class="action-tip">核身已完成，请返回发起核身的页面查看结果</p>

        <div v-if="bizNo" class="biz-no">业务单号：{{ bizNo }}</div>
      </div>
    </div>

    <!-- 结果回流提示 -->
    <div v-if="!terminal" class="foot-note">
      核身完成后将自动返回商户页面。如长时间未跳转，请返回商户页面查看结果。
      <div class="foot-legal">
        <a class="legal-link" :href="`${siteBase('portal')}/auth-authorization`" target="_blank" rel="noopener">实名认证授权协议</a>
        <a class="legal-link" :href="`${siteBase('portal')}/privacy`" target="_blank" rel="noopener">隐私政策</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

/*
 * 站点品牌与站点域名：白标已移除，一律展示平台品牌与平台站点域名
 */
const PLATFORM_NAME = '星楼网络'
const PLATFORM_SITES = {
  portal: 'https://www.starloft.cn',
  console: 'https://console.starloft.cn',
  api: 'https://api.starloft.cn',
  img: 'https://img.starloft.cn',
  service: 'https://service.starloft.cn'
}

const brandName = PLATFORM_NAME

/** 取站点基地址（不含末尾斜杠） */
const siteBase = (kind: keyof typeof PLATFORM_SITES): string => PLATFORM_SITES[kind].replace(/\/$/, '')

document.title = `${brandName} · 人脸核验`

/*
 * FV 认证承接页（自站链接 {service 站点}/fv/:mode）
 *
 * 关于 URL 参数名（前后端契约约定，此处集中定义，便于统一）：
 *   mode       — 核身模式：auth（有源，人脸+公安库比对）/ self（无源，人脸与本人留底比对）。
 *                优先取路由段 :mode，其次取 query 的 mode。
 *   token      — 上游核身令牌（用于拼装上游链接 {上游承接页基址}/{auth|self}?token=xxx）
 *   biz_no     — 平台业务单号（仅展示用）
 *   k          — 备用/签名扩展参数（预留透传）
 *   return_ref — 后端生成的核身完成后的最终回跳地址（GET 方式跳转）
 *   expire_at  — 上游返回的 token 到期时间戳（Unix 秒），用于页面倒计时
 *
 * 若后端返回的即为完整自站链接并带上上述 query 参数，本页直接读取 URL 查询参数即可。
 */
const QUERY_KEYS = {
  token: 'token',
  bizNo: 'biz_no',
  extra: 'k',
  returnRef: 'return_ref',
  expireAt: 'expire_at'
} as const

/**
 * 上游核身承接页基址：由后端公开配置 /console/config 的 fv_upstream_base 下发
 * （形如 https://service.example.com/service/fv），页面按其模式拼接 /auth 或 /self。
 * 未取到时保持空串，页面提示核身通道不可用，避免误跳外部站点。
 */
const upstreamBase = ref('')

// 模式取路由段 + query 兜底
const props = defineProps<{ mode: string }>()
const route = { query: window.location.search ? Object.fromEntries(new URLSearchParams(window.location.search).entries()) : {} }
const runMode = computed<string>(() => {
  const m = String(props.mode || (route.query as Record<string, unknown>).mode || '').toLowerCase()
  return m === 'self' ? 'self' : m === 'auth' ? 'auth' : ''
})

const token = computed(() => (route.query as Record<string, unknown>)[QUERY_KEYS.token] as string | undefined)
const bizNo = computed(() => (route.query as Record<string, unknown>)[QUERY_KEYS.bizNo] as string | undefined)
const returnRef = computed(() => (route.query as Record<string, unknown>)[QUERY_KEYS.returnRef] as string | undefined)

const upstreamUrl = computed(() => {
  if (!token.value || !upstreamBase.value || !runMode.value) return ''
  return `${upstreamBase.value}/${runMode.value}?token=${encodeURIComponent(token.value)}`
})

// 拉取公开配置，取得上游核身承接页基址
onMounted(async () => {
  try {
    const res = await fetch('/console/config')
    const json = await res.json()
    const base = json?.data?.fv_upstream_base
    if (typeof base === 'string' && base) upstreamBase.value = base
  } catch {
    // 取不到配置时保持空串，由页面提示核身通道不可用
  }
})

/** 是否移动端（UA 判断） */
const isMobile = computed(() => {
  const ua = navigator.userAgent || ''
  return /Android|iPhone|iPad|iPod|Mobile|MicroMessenger/i.test(ua)
})

const modeLabel = computed(() => {
  if (runMode.value === 'self') return '无源人脸识别 · 与本人留底照片比对'
  if (runMode.value === 'auth') return '有源人脸识别 · 与公安库信息比对'
  return ''
})

/* 参数完整性校验 */
const errorMsg = computed(() => {
  if (!runMode.value) return '缺少有效的核身模式参数（mode=auth|self）。'
  if (!token.value) return '缺少核身令牌（token）参数，请联系服务方重新发起。'
  if (!upstreamBase.value) return '核身通道未配置（上游平台基址缺失），请联系服务方。'
  return ''
})

/* PC 端二维码（动态 import 本地生成器），生成失败时展示链接兜底 */
const qrSvg = ref('')
const loadQr = async () => {
  if (isMobile.value || errorMsg.value) return
  try {
    const mod = await import('@/utils/qrcode')
    const matrix = mod.generateQRCode(upstreamUrl.value)
    qrSvg.value = mod.qrToSvg(matrix, 4)
  } catch (e) {
    qrSvg.value = ''
  }
}

/* 倒计时：优先用上游返回的 token 到期时间戳（expire_at，Unix 秒）计算剩余秒数，缺失时回退默认值 */
const DEFAULT_TTL = 300
const expireAt = computed(() => Number((route.query as Record<string, unknown>)[QUERY_KEYS.expireAt]) || 0)
const countdown = ref(expireAt.value ? Math.max(0, expireAt.value - Math.floor(Date.now() / 1000)) : DEFAULT_TTL)
let timer: ReturnType<typeof setInterval> | null = null

/* 订单状态轮询：订单进入终态后不再展示二维码 / 上游核身页，切换为认证结果界面。
   轮询只是读平台已落地的状态（结果由 notify_url 回调 / 核身回跳校对落地），
   无回调订单由后端按「次数上限 + 最小间隔」控制向上游同步，前端无需高频轮询，故取 5 秒。 */
const ORDER_FINISHED = 2 // 订单状态 >= 2 均为终态：2已完成 3失败 4已取消 5超时结束 6发起失败
const POLL_INTERVAL = 5000
const REDIRECT_SECONDS = 3

const orderStatus = ref<number | null>(null)
const orderReturnUrl = ref('')
const resultMessage = ref('')
const redirectLeft = ref(REDIRECT_SECONDS)

const terminal = computed(() => orderStatus.value !== null && orderStatus.value >= ORDER_FINISHED)
const success = computed(() => orderStatus.value === 2)
// 终态标题按状态区分：超时结束 / 发起失败并非「认证失败」，不计费无需用户重试付费
const resultTitle = computed(() => {
  switch (orderStatus.value) {
    case 2:
      return '认证成功'
    case 5:
      return '核身已超时'
    case 6:
      return '核验发起失败'
    default:
      return '认证失败'
  }
})
const resultText = computed(() => {
  if (success.value) return '你已完成人脸核验，可以关闭本页面。'
  if (orderStatus.value === 5) return resultMessage.value || '核身链接已超时，未产生扣费，请重新发起核验。'
  if (orderStatus.value === 6) return resultMessage.value || '本次核验发起失败，未产生扣费，请稍后重试。'
  return resultMessage.value || '本次人脸核验未通过，请重新发起。'
})
/* 下游 return_url 为空时停留结果界面；历史数据中的站内相对路径按控制台地址还原 */
const redirectUrl = computed(() => {
  const url = orderReturnUrl.value
  if (!url) return ''
  return url.startsWith('/') ? `${siteBase('console')}${url}` : url
})

let pollTimer: ReturnType<typeof setInterval> | null = null
let redirectTimer: ReturnType<typeof setInterval> | null = null

const stopPoll = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

/* 认证成功后按 return_url 倒计时跳转；无 return_url 则停留在结果界面 */
const startRedirect = () => {
  if (!redirectUrl.value || redirectTimer) return
  redirectLeft.value = REDIRECT_SECONDS
  redirectTimer = setInterval(() => {
    redirectLeft.value -= 1
    if (redirectLeft.value <= 0) {
      if (redirectTimer) clearInterval(redirectTimer)
      redirectTimer = null
      window.location.href = redirectUrl.value
    }
  }, 1000)
}

/* force=true：用户点击「我已完成核身」时请求后端立即校对一次上游结果 */
const pollStatus = async (force = false) => {
  if (terminal.value || !token.value) return
  try {
    const url = `/service/fv/status?token=${encodeURIComponent(token.value)}${force ? '&sync=1' : ''}`
    const res = await fetch(url)
    const json = await res.json()
    if (json.code === 404) {
      stopPoll() // 令牌对应的订单不存在（已过期/无效），停止轮询
      return
    }
    if (json.code !== 0) return
    orderStatus.value = Number(json.data.status)
    orderReturnUrl.value = String(json.data.return_url || '')
    resultMessage.value = String(json.data.result_message || '')
    if (terminal.value) {
      stopPoll()
      startRedirect()
    }
  } catch (e) {
    // 网络异常时保持轮询，等待下次重试
  }
}

/* 我已完成核身：先同步一次订单状态；有回跳地址则跳转，否则返回来源页或停留结果界面 */
const finishedTip = ref(false)
const finish = async () => {
  await pollStatus(true)
  const target = redirectUrl.value || returnRef.value
  if (target) {
    window.location.href = target
    return
  }
  if (terminal.value) return
  if (window.history.length > 1) {
    window.history.back()
    return
  }
  finishedTip.value = true
}

/* 复制认证链接：优先用剪贴板 API，不可用时回退到临时输入框选中复制 */
const copied = ref(false)
const copyLink = async () => {
  const link = upstreamUrl.value
  if (!link) return
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(link)
    } else {
      const el = document.createElement('textarea')
      el.value = link
      el.style.position = 'fixed'
      el.style.opacity = '0'
      document.body.appendChild(el)
      el.select()
      document.execCommand('copy')
      document.body.removeChild(el)
    }
    copied.value = true
    setTimeout(() => (copied.value = false), 3000)
  } catch (e) {
    copied.value = false
  }
}

onMounted(() => {
  if (!errorMsg.value) {
    loadQr()
    pollStatus()
    pollTimer = setInterval(pollStatus, POLL_INTERVAL)
  }
  if (!isMobile.value && !errorMsg.value) {
    timer = setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0) {
        countdown.value = 0
        if (timer) clearInterval(timer)
      }
    }, 1000)
  }
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  stopPoll()
  if (redirectTimer) clearInterval(redirectTimer)
})
</script>

<style scoped>
.fv-verify {
  min-height: 100vh;
  background: linear-gradient(135deg, #15C5BA 0%, #1CD5C7 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  font-family: -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
.err-page,
.mobile-proxy,
.pc-page,
.result-page {
  width: 100%;
  max-width: 420px;
  background: #fff;
  border-radius: 16px;
  padding: 40px 32px;
  text-align: center;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
}
/* 认证结果界面 */
.result-icon {
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  border-radius: 50%;
  font-size: 30px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.result-icon.ok {
  background: rgba(28, 213, 199, 0.12);
  color: var(--color-primary, #1CD5C7);
}
.result-icon.fail {
  background: var(--color-danger-light, #fde8e8);
  color: var(--color-danger, #e34d59);
}
.redirect-tip {
  color: #9ca3af;
  font-size: 13px;
}
.err-icon {
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  border-radius: 50%;
  background: var(--color-danger-light, #fde8e8);
  color: var(--color-danger, #e34d59);
  font-size: 30px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
h1 {
  font-size: 22px;
  color: #1f2937;
  margin: 0 0 12px;
}
p {
  color: #6b7280;
  font-size: 14px;
  line-height: 1.7;
  margin: 0 0 16px;
}
.mode-tip {
  color: var(--color-primary, #1CD5C7);
  font-weight: 500;
  margin-bottom: 20px;
}
/* 移动端代理 iframe */
.mobile-proxy {
  max-width: 480px;
  padding: 0;
  overflow: hidden;
}
.proxy-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-size: 13px;
  color: #374151;
  background: #f9fafb;
  border-bottom: 1px solid #e5e7eb;
}
.proxy-open {
  color: var(--color-primary, #1CD5C7);
  text-decoration: none;
  font-weight: 600;
}
.proxy-frame {
  width: 100%;
  height: calc(100vh - 300px);
  min-height: 420px;
  border: none;
  display: block;
}
.proxy-footer {
  padding: 14px 16px 18px;
}
.qr-wrap {
  margin-bottom: 12px;
}
.qr-box {
  width: 240px;
  height: 240px;
  margin: 0 auto;
  padding: 16px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
}
.qr-box :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}
.qr-alt .link {
  display: block;
  word-break: break-all;
  color: var(--color-primary, #1CD5C7);
  font-size: 13px;
}
.scan-tip {
  font-size: 15px;
  color: #374151;
  font-weight: 500;
  margin: 8px 0;
}
.countdown {
  color: #9ca3af;
  font-size: 13px;
  margin-bottom: 20px;
}
.count {
  color: #e34d59;
  font-weight: 700;
}
.actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.btn {
  display: block;
  padding: 11px 0;
  border-radius: 8px;
  background: var(--color-primary, #1CD5C7);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  text-align: center;
  text-decoration: none;
  border: none;
  cursor: pointer;
}
.btn:hover {
  opacity: 0.92;
}
.btn.ghost {
  background: #fff;
  color: var(--color-primary, #1CD5C7);
  border: 1px solid var(--color-primary, #1CD5C7);
}
.action-tip {
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--color-primary, #1CD5C7);
}
.biz-no {
  margin-top: 16px;
  font-size: 12px;
  color: #9ca3af;
}
.foot-note {
  margin-top: 20px;
  color: rgba(255, 255, 255, 0.85);
  font-size: 13px;
  text-align: center;
  max-width: 420px;
}
.foot-legal {
  margin-top: 8px;
  display: flex;
  justify-content: center;
  gap: 16px;
}
.foot-legal .legal-link {
  color: rgba(255, 255, 255, 0.7);
  text-decoration: underline;
  transition: color 0.2s;
}
.foot-legal .legal-link:hover {
  color: #fff;
}
</style>