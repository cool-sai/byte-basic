/** 通用 module→detail 壳的 detail.data 约定（一期）。 */
export interface GenericDetailData {
  /** 列表摘要 */
  summary?: string
  /** 列表/详情展示用日期 YYYY-MM-DD */
  date?: string
  /** Markdown 正文（优先） */
  md?: string
  /** Markdown 正文别名 */
  content?: string
  /** 旧 life 形分区（无 md/content 时兜底） */
  sections?: GenericSection[]
}

export interface GenericSection {
  key?: string
  title?: string
  items?: GenericSectionItem[]
}

export interface GenericSectionItem {
  text?: string
  link?: string
}

/** 从 pathname 取 module slug：/demo/ → demo，/demo/detail.html → demo。 */
export function moduleSlugFromPath(pathname = location.pathname): string {
  const parts = pathname.replace(/\/+$/, "").split("/").filter(Boolean)
  // detail.html 不是模块名
  if (parts.length >= 1 && !parts[0]!.includes(".")) return parts[0]!
  return ""
}

export function markdownOf(data: GenericDetailData | undefined): string | null {
  if (!data) return null
  if (typeof data.md === "string" && data.md.trim()) return data.md
  if (typeof data.content === "string" && data.content.trim()) return data.content
  return null
}
