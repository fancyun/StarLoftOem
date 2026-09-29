import axios from 'axios'
import { useUserStore } from '@/stores/user'

// 从对外图片地址中取出上传目录下的相对路径：
// 新地址为 img.starloft.cn/uploads/{日期}/{md5}.jpg，历史地址为 console.starloft.cn/console/uploads/...
export const uploadRelPath = (url: string): string => {
  if (!url) return ''
  const idx = url.indexOf('/uploads/')
  if (idx < 0) return ''
  return url.slice(idx + '/uploads/'.length).replace(/^\/+/, '')
}

// 资质图受 UploadAuth 保护，浏览器 <img> 直连不带鉴权会 403：
// 统一改走同域 /img/uploads/*（由控制台站点反代到后端 /img/uploads/* 校验归属），
// 携带 user token 拉 blob 后转为 object URL。调用方在不再使用时需 revokeObjectURL 释放。
export const loadAuthImage = async (url: string): Promise<string> => {
  const relPath = uploadRelPath(url)
  if (!relPath) return ''
  const userStore = useUserStore()
  const resp = await axios.get(`/img/uploads/${relPath}`, {
    responseType: 'blob',
    timeout: 15000,
    headers: { Authorization: `Bearer ${userStore.token}` }
  })
  return URL.createObjectURL(resp.data)
}

// 上传前压缩阈值：超过该大小的图片走 canvas 重编码（默认 1MB，为避开 Nginx 默认 1MB 请求体限制）
const UPLOAD_COMPRESS_MAX_SIZE = 1024 * 1024
// 压缩后最长边（保留足够文字辨识度，营业执照/身份证等资质材料需可辨认）
const UPLOAD_COMPRESS_MAX_EDGE = 1920

// 解码图片为可绘制对象（data URL 方式，兼容性最好）
const decodeImage = (file: File): Promise<{ source: HTMLImageElement; width: number; height: number }> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('图片读取失败'))
    reader.onload = () => {
      const img = new Image()
      img.onload = () => resolve({ source: img, width: img.naturalWidth, height: img.naturalHeight })
      img.onerror = () => reject(new Error('图片解码失败'))
      img.src = String(reader.result)
    }
    reader.readAsDataURL(file)
  })

const canvasToJpeg = (canvas: HTMLCanvasElement, quality: number): Promise<Blob | null> =>
  new Promise((resolve) => canvas.toBlob((blob) => resolve(blob), 'image/jpeg', quality))

// 上传前按需压缩：未超过 maxSize 的原图原样返回；超过则重编码为 JPEG（限制最长边 + 质量逐级下探）。
// 解码/编码失败时返回原图，由后端按原有规则校验，不阻断上传流程。
export const compressImageIfNeeded = async (
  file: File,
  maxSize: number = UPLOAD_COMPRESS_MAX_SIZE,
  maxEdge: number = UPLOAD_COMPRESS_MAX_EDGE
): Promise<File> => {
  if (file.size <= maxSize) return file

  try {
    const { source, width, height } = await decodeImage(file)
    const scale = Math.min(1, maxEdge / Math.max(width, height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(width * scale))
    canvas.height = Math.max(1, Math.round(height * scale))

    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    // JPEG 无透明通道，透明区域填白，避免 PNG 转码后出现黑底
    ctx.fillStyle = '#FFFFFF'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(source, 0, 0, canvas.width, canvas.height)

    let smallest: Blob | null = null
    for (const quality of [0.85, 0.75, 0.65, 0.5, 0.4]) {
      const blob = await canvasToJpeg(canvas, quality)
      if (!blob) return file
      smallest = blob
      if (blob.size <= maxSize) break
    }
    if (!smallest) return file
    // 重编码后为 JPEG，文件名扩展名同步改为 .jpg（后端按扩展名白名单校验）
    const name = file.name.replace(/\.[^.]+$/, '') + '.jpg'
    return new File([smallest], name, { type: 'image/jpeg' })
  } catch (error) {
    console.error('图片压缩失败，按原图上传:', error)
    return file
  }
}