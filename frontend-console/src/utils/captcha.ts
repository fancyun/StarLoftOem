/**
 * 人机验证码前端封装：支持腾讯天御 / 极验行为验证码 / 阿里云行为验证码，
 * 具体通道与渲染参数由后端 /console/config 的 captcha 字段下发，切换通道无需改前端代码。
 */

import { publicAPI } from '@/api'

// 全局类型声明（三方 SDK 全局对象）
declare global {
  interface Window {
    TencentCaptcha: any
    initGeetest4: any
    AliyunCaptcha: any
  }
}

/** 验证码通道配置（后端下发） */
interface CaptchaConfig {
  provider: string // tencent / geetest / aliyun
  app_id?: string // 腾讯天御 CaptchaAppId
  captcha_id?: string // 极验 CaptchaId
  scene_id?: string // 阿里云场景 ID
}

/** 前端验证结果：通道标识 + 该通道的服务端校验参数 */
export interface CaptchaResultPayload {
  provider: string
  payload: Record<string, string>
}

// 缓存通道配置，避免重复请求
let cachedConfig: CaptchaConfig | null = null

// 重置缓存
export function resetCaptchaCache() {
  cachedConfig = null
}

/** 从后端获取验证码通道配置 */
async function getCaptchaConfig(): Promise<CaptchaConfig> {
  if (cachedConfig) return cachedConfig

  const config: any = await publicAPI.getConfig()
  const c = config?.captcha
  if (c && c.provider) {
    cachedConfig = c as CaptchaConfig
  } else {
    // 兼容未升级后端：回落腾讯天御 + 旧的 captcha_app_id 字段
    cachedConfig = { provider: 'tencent', app_id: config?.captcha_app_id }
  }
  if (cachedConfig.provider === 'tencent' && !cachedConfig.app_id) {
    throw new Error('验证码配置缺失（腾讯天御 CaptchaAppId 未配置）')
  }
  return cachedConfig
}

/** 动态加载脚本（已加载或加载中时直接复用） */
function loadScript(src: string, marker: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector(`script[src*="${marker}"]`)
    if (existing) {
      existing.addEventListener('load', () => resolve())
      existing.addEventListener('error', () => reject(new Error('验证码 SDK 加载失败')))
      // 已加载完成但未触发事件时，下一次微任务直接放行
      setTimeout(() => resolve(), 0)
      return
    }
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('验证码 SDK 加载失败'))
    document.head.appendChild(script)
  })
}

/** 腾讯天御验证码 2.0 */
async function runTencentCaptcha(appId: string): Promise<CaptchaResultPayload> {
  await loadScript('https://turing.captcha.qcloud.com/TJCaptcha.js', 'TJCaptcha.js')
  return new Promise((resolve, reject) => {
    try {
      const captcha = new window.TencentCaptcha(
        appId,
        (res: any) => {
          if (res.ret === 0) {
            resolve({ provider: 'tencent', payload: { ticket: res.ticket, randstr: res.randstr } })
          } else if (res.ret === 2) {
            reject(new Error('用户取消验证'))
          } else {
            reject(new Error(res.errorMessage || '验证失败'))
          }
        },
        { needFeedBack: false }
      )
      captcha.show()
    } catch (error) {
      reject(new Error('验证码初始化失败: ' + (error as Error).message))
    }
  })
}

/** 极验行为验证码 v4 */
async function runGeetestCaptcha(captchaId: string): Promise<CaptchaResultPayload> {
  await loadScript('https://static.geetest.com/v4/gt4.js', 'gt4.js')
  return new Promise((resolve, reject) => {
    if (!window.initGeetest4) {
      reject(new Error('极验 SDK 加载失败'))
      return
    }
    try {
      window.initGeetest4({ captchaId, product: 'bind' }, (captcha: any) => {
        captcha
          .onSuccess(() => {
            const v = captcha.getValidate() || {}
            resolve({
              provider: 'geetest',
              payload: {
                lot_number: String(v.lot_number || ''),
                captcha_output: String(v.captcha_output || ''),
                pass_token: String(v.pass_token || ''),
                gen_time: String(v.gen_time || '')
              }
            })
          })
          .onError((e: any) => reject(new Error('验证失败: ' + (e?.msg || ''))))
          .onClose(() => reject(new Error('用户取消验证')))
        captcha.showCaptcha()
      })
    } catch (error) {
      reject(new Error('验证码初始化失败: ' + (error as Error).message))
    }
  })
}

/**
 * 阿里云行为验证码 2.0。
 * 该 SDK 采用「回调式」集成：验证通过后在 captchaVerifyCallback 中拿到 captchaVerifyParam，
 * 本系统统一由后端接口校验（与其它通道一致），故此处仅取出参数并交由业务请求上送。
 */
async function runAliyunCaptcha(sceneId: string): Promise<CaptchaResultPayload> {
  await loadScript(
    'https://o.alicdn.com/captcha-frontend/aliyunCaptcha/AliyunCaptcha.js',
    'AliyunCaptcha.js'
  )
  return new Promise((resolve, reject) => {
    if (!window.AliyunCaptcha || !sceneId) {
      reject(new Error('阿里云验证码未配置'))
      return
    }
    const holderId = 'aliyun-captcha-holder'
    let holder = document.getElementById(holderId)
    if (!holder) {
      holder = document.createElement('div')
      holder.id = holderId
      document.body.appendChild(holder)
    }
    try {
      window.AliyunCaptcha({
        SceneId: sceneId,
        prefix: 'oem',
        mode: 'popup',
        element: `#${holderId}`,
        captchaVerifyCallback: async (captchaVerifyParam: string) => {
          resolve({ provider: 'aliyun', payload: { captcha_verify_param: captchaVerifyParam } })
          // 返回通过以关闭弹层：真正的服务端校验在业务请求中由后端完成
          return { captchaResult: true }
        },
        onBizResultCallback: () => {},
        getInstance: () => {},
        slideStyle: { width: 360, height: 40 },
        language: 'cn'
      })
    } catch (error) {
      reject(new Error('验证码初始化失败: ' + (error as Error).message))
    }
  })
}

/**
 * 触发人机验证码并返回该通道的服务端校验参数。
 * 使用方式：const captcha = await verifyCaptcha(); 请求体带上 captcha_payload: captcha.payload
 */
export async function verifyCaptcha(): Promise<CaptchaResultPayload> {
  const config = await getCaptchaConfig()
  switch (config.provider) {
    case 'geetest':
      return runGeetestCaptcha(config.captcha_id || '')
    case 'aliyun':
      return runAliyunCaptcha(config.scene_id || '')
    default:
      return runTencentCaptcha(config.app_id || '')
  }
}