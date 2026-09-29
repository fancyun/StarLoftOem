/**
 * 格式化后端返回的日期时间字符串。
 * 去除末尾时区信息（如 +08:00 / Z / +0800），并将 T 分隔符统一为空格。
 * 返回 "YYYY-MM-DD HH:mm:ss" 格式，若无值则返回 '-'。
 */
export function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  let s = String(value).trim()
  if (s === '') return '-'

  // 统一 T 分隔符为空格
  s = s.replace('T', ' ')

  // 去除毫秒部分
  s = s.replace(/\.\d+/, '')

  // 去除末尾时区：+08:00 / +0800 / -05:00 / Z
  s = s.replace(/\s*[+-]\d{2}:?\d{2}$/, '')
  s = s.replace(/\s*Z$/i, '')

  return s
}