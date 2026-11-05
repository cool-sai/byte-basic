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
  /** templates：Mustache HTML（:::render 拉取） */
  html?: string
  /** templates：详情预览默认 mock；缺省 {} */
  mock?: unknown
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

/**
 * templates 模块下：无 md/content 但有 data.html 时，默认挂 preview 模板，
 * 走 :::render 展示 Mock / 模板源码 / 渲染效果（不另起专用详情 UI）。
 */
export function detailMarkdown(
  data: GenericDetailData | undefined,
  moduleSlug: string,
  detailSlug: string,
): string | null {
  const md = markdownOf(data)
  if (md) return md
  if (
    moduleSlug === "templates" &&
    detailSlug &&
    detailSlug !== "preview" &&
    typeof data?.html === "string" &&
    data.html.trim()
  ) {
    return `:::render{template="preview" of="${detailSlug}"}\n:::\n`
  }
  return null
}
