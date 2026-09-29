import { publicAPI } from '@/api'

// 在线支付渠道（value 与后端 payment_order.channel 取值一致）
export const PAY_CHANNEL_OPTIONS = [
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信支付' }
]

// 渠道中文名（含人工支付，用于支付详情展示）
export const PAY_CHANNEL_LABELS: Record<string, string> = {
  alipay: '支付宝',
  wechat: '微信支付',
  manual: '人工支付'
}

// 拉取平台已启用的在线支付渠道（未配置凭据的渠道不展示）；拉取失败时回退为支付宝/微信
export async function loadEnabledPayChannels(): Promise<string[]> {
  try {
    const config: any = await publicAPI.getConfig()
    if (Array.isArray(config?.payment_channels) && config.payment_channels.length) {
      return config.payment_channels
    }
  } catch (error) {
    console.error(error)
  }
  return ['alipay', 'wechat']
}