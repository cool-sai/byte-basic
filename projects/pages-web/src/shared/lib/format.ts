const nf = new Intl.NumberFormat("zh-CN")
export const fmtNum = (v: number) => nf.format(v)

/** board 时间是北京时间 "YYYY-MM-DD HH:mm:ss"，展示时去掉秒。 */
export const fmtTime = (s?: string) => (s ? s.slice(0, 16) : "")

/** 把字段名转成人话：snake_case → 空格分隔；已有中文的原样返回。 */
export const humanKey = (k: string) => (/[\u4e00-\u9fa5]/.test(k) ? k : k.replace(/_/g, " "))

export function hostOf(url: string): string {
  try { return new URL(url).hostname.replace(/^www\./, "") } catch { return "" }
}

/** 可读短时间（Asia/Shanghai）：10/18 22:11。接受 board "YYYY-MM-DD HH:mm:ss"、带 CST、或 ISO。 */
export function fmtShortTime(s?: string | null): string {
  if (!s) return ""
  const m = s.trim().match(/(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2})/)
  if (!m) return s.slice(0, 16)
  return `${Number(m[2])}/${Number(m[3])} ${m[4]}:${m[5]}`
}

/** 排序用毫秒；解析失败返回 0。 */
export function timeMs(s?: string | null): number {
  if (!s) return 0
  const t = s.trim().replace(" CST", "").replace(" ", "T")
  // board 无时区时按 +08:00
  const withTz = /[Zz]|[+-]\d{2}:?\d{2}$/.test(t) ? t : t + "+08:00"
  const n = Date.parse(withTz)
  return Number.isFinite(n) ? n : 0
}
