/**
 * 腾讯云天御验证码 2.0 封装
 * 官方文档: https://cloud.tencent.com/document/product/1110/36841
 */

import { publicAPI } from '@/api'

// 全局类型声明
declare global {
  interface Window {
    TencentCaptcha: any
  }
}

export interface CaptchaResult {
  ret: number          // 0: 验证成功, 2: 用户主动关闭
  ticket: string       // 验证成功的票据（ret=0时有效），可能包含 trerror_ 前缀（容灾票据）
  randstr: string      // 随机串（后续票据校验需要）
  errorCode?: number   // 错误码
  errorMessage?: string // 错误信息
  CaptchaAppId?: string // 验证码应用ID
  bizState?: any       // 自定义透传参数
}

// 缓存 CaptchaAppId，避免重复请求
let cachedCaptchaAppId: string | null = null

// 重置缓存
export function resetCaptchaCache() {
  cachedCaptchaAppId = null
}

/**
 * 从后端获取 CaptchaAppId
 */
async function getCaptchaAppId(): Promise<string> {
  if (cachedCaptchaAppId) {
    return cachedCaptchaAppId
  }
  
  try {
    const config: any = await publicAPI.getConfig()
    
    // request 响应拦截器已经返回了 response.data，所以 config 就是后端返回的 data 对象
    cachedCaptchaAppId = config.captcha_app_id
    
    if (!cachedCaptchaAppId) {
      console.error('配置中未找到 captcha_app_id，完整配置:', JSON.stringify(config))
      throw new Error('配置中未找到 captcha_app_id')
    }
    
    return cachedCaptchaAppId
  } catch (error) {
    console.error('获取验证码配置失败:', error)
    throw new Error('无法获取验证码配置: ' + (error as Error).message)
  }
}

/**
 * 动态加载腾讯验证码 SDK
 */
function loadCaptchaScript(): Promise<void> {
  return new Promise((resolve, reject) => {
    // 如果已加载，直接返回
    if (window.TencentCaptcha) {
      resolve()
      return
    }

    // 检查是否已经有 script 标签正在加载
    const existingScript = document.querySelector('script[src*="TJCaptcha.js"]')
    if (existingScript) {
      existingScript.addEventListener('load', () => resolve())
      existingScript.addEventListener('error', () => reject(new Error('验证码 SDK 加载失败')))
      return
    }

    // 动态创建 script 标签
    const script = document.createElement('script')
    script.src = 'https://turing.captcha.qcloud.com/TJCaptcha.js'  // 验证码2.0 JS地址
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('验证码 SDK 加载失败'))
    document.head.appendChild(script)
  })
}

/**
 * 显示腾讯天御验证码
 * @param appId - 从后端获取的 CaptchaAppId（字符串）
 * @returns Promise<CaptchaResult>
 */
async function showTencentCaptcha(appId: string): Promise<CaptchaResult> {
  // 1. 确保 SDK 已加载
  await loadCaptchaScript()

  return new Promise((resolve, reject) => {
    try {
      // 2. 验证 appId 是否为有效的非空字符串
      if (!appId || typeof appId !== 'string' || appId.trim() === '') {
        console.error('CaptchaAppId 无效:', appId)
        reject(new Error('CaptchaAppId 格式不正确'))
        return
      }

      // 3. 创建验证码实例并显示（TencentCaptcha 构造函数第一个参数为字符串类型的 CaptchaAppId）
      const captcha = new window.TencentCaptcha(appId, (res: CaptchaResult) => {
        // 回调函数处理验证结果
        if (res.ret === 0) {
          // 验证成功（包括正常票据和容灾票据）
          // 注意：ticket 包含 trerror_ 前缀时表示容灾票据，
          // 这是因为用户网络较差导致前端自动容灾生成的票据
          // 后端应该在票据校验时根据业务需求进行处理
          resolve(res)
        } else if (res.ret === 2) {
          // 用户主动关闭
          reject(new Error('用户取消验证'))
        } else {
          // 其他错误
          reject(new Error(res.errorMessage || '验证失败'))
        }
      }, {
        // 可选配置项
        needFeedBack: false,  // 不显示用户反馈按钮
        // type: 'popup',     // popup(弹窗) 或 embed(内嵌)，默认 popup
        // loading: true,     // 显示加载动画
      })

      // 显示验证码
      captcha.show()

    } catch (error) {
      reject(new Error('验证码初始化失败: ' + (error as Error).message))
    }
  })
}

/**
 * 登录前触发验证码
 * @param appId - CaptchaAppId
 * @returns Promise<{ ticket: string, randstr: string }>
 */
async function verifyCaptchaForLogin(appId: string): Promise<{ ticket: string, randstr: string }> {
  const result = await showTencentCaptcha(appId)
  return {
    ticket: result.ticket,
    randstr: result.randstr
  }
}

/**
 * 快捷方法：从后端 API 获取 CaptchaAppId 并显示验证码
 * 使用方式：
 * import { verifyCaptcha } from '@/utils/captcha'
 * const { ticket, randstr } = await verifyCaptcha()
 */
export async function verifyCaptcha(): Promise<{ ticket: string, randstr: string }> {
  const appId = await getCaptchaAppId()
  return verifyCaptchaForLogin(appId)
}
