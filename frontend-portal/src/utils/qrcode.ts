// 本地 QR 码生成器（字节模式，纠错级别 M，版本 1–6）
// 自包含、无第三方依赖，用于把上游核身链接编码为二维码。
// 参考 ISO/IEC 18004 标准实现：含 finder/timing/alignment 功能图形、格式信息、RS 纠错与掩码。

/* ========== GF(256) 运算表（原多项式 0x11d） ========== */
const EXP = new Array<number>(512)
const LOG = new Array<number>(256)
;(() => {
  let v = 1
  for (let i = 0; i < 255; i++) {
    EXP[i] = v
    LOG[v] = i
    v <<= 1
    if (v & 0x100) v ^= 0x11d
  }
  for (let i = 255; i < 512; i++) EXP[i] = EXP[i - 255]
})()

const gmul = (a: number, b: number): number => {
  if (a === 0 || b === 0) return 0
  return EXP[(LOG[a] + LOG[b]) % 255]
}

/* ========== 纠错块配置（仅纠错级别 M） ==========
 * 每项：[块数, 每块总码字, 每块数据码字]（本工程仅用于 v1–6，均为单组块结构）
 */
const RS_BLOCKS_M: Record<number, { count: number; total: number; data: number }> = {
  1: { count: 1, total: 26, data: 16 },
  2: { count: 1, total: 44, data: 28 },
  3: { count: 1, total: 70, data: 44 },
  4: { count: 2, total: 50, data: 32 },
  5: { count: 2, total: 67, data: 43 },
  6: { count: 4, total: 43, data: 27 }
}

/* 对齐图形中心坐标（v2–6） */
const ALIGN_CENTERS: Record<number, number[]> = {
  2: [6, 18],
  3: [6, 22],
  4: [6, 26],
  5: [6, 30],
  6: [6, 34]
}

/** 根据数据字节长度选择最小可用版本；超长返回 0 */
const pickVersion = (byteLen: number): number => {
  for (const [ver, cfg] of Object.entries(RS_BLOCKS_M)) {
    const dataCodewords = cfg.data * cfg.count
    // 可用比特 = 4(mode) + 8(count, v1–9) + 8*len
    if (4 + 8 + byteLen * 8 <= dataCodewords * 8) return Number(ver)
  }
  return 0
}

/* ========== Reed-Solomon 纠错 ========== */
const genPoly = (nsym: number): number[] => {
  let g: number[] = [1]
  for (let i = 0; i < nsym; i++) {
    // 乘以 (x + α^i)
    const next = new Array<number>(g.length + 1).fill(0)
    for (let j = 0; j < g.length; j++) {
      next[j] ^= g[j]
      next[j + 1] ^= gmul(g[j], EXP[i])
    }
    g = next
  }
  return g
}

const reedSolomon = (data: number[], nsym: number): number[] => {
  const gen = genPoly(nsym)
  const res = new Array<number>(data.length + nsym).fill(0)
  data.forEach((b, i) => (res[i] = b))
  for (let i = 0; i < data.length; i++) {
    const coef = res[i]
    if (coef !== 0) {
      for (let j = 0; j < gen.length; j++) {
        res[i + j] ^= gmul(gen[j], coef)
      }
    }
  }
  return res.slice(data.length)
}

/* ========== 格式信息（纠错级别 M → 00，掩码 0） ========== */
const BCH_G15 = 0b10100110111 // 0x537
const BCH_MASK = 0b101010000010010 // 0x5412

const getBCHTypeInfo = (data: number): number => {
  let d = data << 10
  while (Math.floor(Math.log2(d)) - Math.floor(Math.log2(BCH_G15)) >= 0) {
    d ^= BCH_G15 << (Math.floor(Math.log2(d)) - Math.floor(Math.log2(BCH_G15)))
  }
  return ((data << 10) | d) ^ BCH_MASK
}

/* ========== 生成二维码布尔矩阵 ========== */
export interface QRMatrix {
  size: number
  version: number
  /** true=深色 / false=浅色 */
  modules: boolean[][]
}

/**
 * 生成二维码矩阵。字节过长（>v6 M 容量约 106 字节）时抛出错误。
 */
export const generateQRCode = (text: string): QRMatrix => {
  const bytes = new TextEncoder().encode(text)
  const version = pickVersion(bytes.length)
  if (version === 0) {
    throw new Error(`内容过长，超出支持容量（${bytes.length} 字节）`)
  }
  const cfg = RS_BLOCKS_M[version]
  const moduleCount = version * 4 + 17
  const size = moduleCount

  /* 1) 构造数据序列：模式(0100) + 长度(8bit) + 数据 + 终止符 + 填充码字 */
  const capacityBits = cfg.data * cfg.count * 8
  const bits: number[] = []
  const pushBits = (val: number, n: number) => {
    for (let b = n - 1; b >= 0; b--) bits.push((val >> b) & 1)
  }
  pushBits(0b0100, 4) // 字节模式
  pushBits(bytes.length, 8)
  for (const b of bytes) pushBits(b, 8)
  // 终止符（最多 4 位 0）
  const remain = capacityBits - bits.length
  const term = Math.min(remain, 4)
  pushBits(0, term)
  // 填充到位（每字节 0xEC / 0x11 交替）
  const padBytes = Math.floor((capacityBits - bits.length) / 8)
  for (let i = 0; i < padBytes; i++) pushBits(i % 2 === 0 ? 0xec : 0x11, 8)
  // 剩余位（remainder bits）置 0
  while (bits.length < capacityBits) bits.push(0)

  // 2) 数据码字分块
  const dataCodewords: number[] = []
  for (let i = 0; i < bits.length; i += 8) {
    let b = 0
    for (let j = 0; j < 8; j++) b = (b << 1) | bits[i + j]
    dataCodewords.push(b)
  }
  const eccPerBlock = cfg.total - cfg.data
  const blockData: number[][] = []
  const blockEcc: number[][] = []
  for (let b = 0; b < cfg.count; b++) {
    const chunk = dataCodewords.slice(b * cfg.data, (b + 1) * cfg.data)
    blockData.push(chunk)
    blockEcc.push(reedSolomon(chunk, eccPerBlock))
  }
  // 交织数据码字（各块等长，按位纵向取）
  const interleavedData: number[] = []
  for (let i = 0; i < cfg.data; i++) {
    for (let b = 0; b < cfg.count; b++) interleavedData.push(blockData[b][i])
  }
  const interleavedEcc: number[] = []
  for (let i = 0; i < eccPerBlock; i++) {
    for (let b = 0; b < cfg.count; b++) interleavedEcc.push(blockEcc[b][i])
  }
  const codewordStream = interleavedData.concat(interleavedEcc)

  // 3) 构建功能图形并得到 reserved 标记
  const modules: boolean[][] = []
  const reserved: boolean[][] = []
  for (let r = 0; r < size; r++) {
    modules[r] = new Array<boolean>(size).fill(false)
    reserved[r] = new Array<boolean>(size).fill(false)
  }
  const set = (r: number, c: number, val: boolean, isReserved: boolean) => {
    modules[r][c] = val
    if (isReserved) reserved[r][c] = true
  }

  // finder 图案（7x7）+ 分隔符
  const placeFinder = (row: number, col: number) => {
    for (let r = -1; r <= 7; r++) {
      for (let c = -1; c <= 7; c++) {
        const rr = row + r
        const cc = col + c
        if (rr < 0 || rr >= size || cc < 0 || cc >= size) continue
        const dark =
          (r >= 0 && r <= 6 && (c === 0 || c === 6)) ||
          (c >= 0 && c <= 6 && (r === 0 || r === 6)) ||
          (r >= 2 && r <= 4 && c >= 2 && c <= 4)
        set(rr, cc, dark, true)
      }
    }
  }
  placeFinder(0, 0)
  placeFinder(0, size - 7)
  placeFinder(size - 7, 0)

  // 时序图形
  for (let i = 8; i < size - 8; i++) {
    const dark = i % 2 === 0
    set(6, i, dark, true)
    set(i, 6, dark, true)
  }

  // 对齐图形
  const centers = ALIGN_CENTERS[version] || []
  for (const cy of centers) {
    for (const cx of centers) {
      // 与 finder 重叠的角点跳过
      if (
        (cy <= 8 && cx <= 8) ||
        (cy <= 8 && cx >= size - 9) ||
        (cy >= size - 9 && cx <= 8)
      ) {
        continue
      }
      for (let r = -2; r <= 2; r++) {
        for (let c = -2; c <= 2; c++) {
          const dark = Math.max(Math.abs(r), Math.abs(c)) !== 1
          set(cy + r, cx + c, dark, true)
        }
      }
    }
  }

  // 深色模块（固定）
  set(size - 8, 8, true, true)

  // 4) 数据放置（掩码 0）
  const mask0 = (x: number, y: number) => (x + y) % 2 === 0
  let bitIndex = 0
  const isDark = (value: boolean, x: number, y: number) => value !== mask0(x, y)
  // 从右下起按 Z 字形填充
  let upward = true
  let col = size - 1
  while (col >= 1) {
    if (col === 6) col = 5
    const rowStart = upward ? size - 1 : 0
    const delta = upward ? -1 : 1
    for (let r = rowStart; r >= 0 && r < size; r += delta) {
      for (let c = 0; c < 2; c++) {
        const x = col - c
        if (x < 0 || reserved[r][x]) continue
        const byteIdx = bitIndex >> 3
        const bitPos = 7 - (bitIndex & 7)
        const bit = (codewordStream[byteIdx] >> bitPos) & 1
        set(r, x, isDark(bit === 1, x, r), false)
        bitIndex++
      }
    }
    upward = !upward
    col -= 2
  }

  // 5) 格式信息
  const fmt = getBCHTypeInfo(0) // 纠错 M(00) << 3 | 掩码0 = 0
  const fmtBit = (i: number) => ((fmt >> i) & 1) === 1
  // 左上角：纵向
  for (let i = 0; i < 15; i++) {
    const mod = fmtBit(i)
    if (i < 6) set(i, 8, mod, true)
    else if (i < 8) set(i + 1, 8, mod, true)
    else set(size - 15 + i, 8, mod, true)
  }
  // 横向
  for (let i = 0; i < 15; i++) {
    const mod = fmtBit(i)
    if (i < 8) set(8, size - i - 1, mod, true)
    else if (i < 9) set(8, 15 - i - 1 + 1, mod, true)
    else set(8, 15 - i - 1, mod, true)
  }
  // 固定深色
  set(size - 8, 8, true, true)

  return { size, version, modules }
}

/** 由矩阵生成内联 SVG 字符串（含一圈浅色静区） */
export const qrToSvg = (matrix: QRMatrix, moduleSize = 4): string => {
  const quiet = 4
  const content = matrix.size * moduleSize
  const canvas = content + quiet * 2 * moduleSize
  const pos = (v: number) => quiet * moduleSize + v * moduleSize
  let rects = ''
  for (let r = 0; r < matrix.size; r++) {
    for (let c = 0; c < matrix.size; c++) {
      if (matrix.modules[r][c]) {
        rects += `<rect x="${pos(c)}" y="${pos(r)}" width="${moduleSize}" height="${moduleSize}"/>`
      }
    }
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${canvas} ${canvas}" shape-rendering="crispEdges"><rect width="${canvas}" height="${canvas}" fill="#fff"/><g fill="#111">${rects}</g></svg>`
}