import Mustache from "mustache"
import DOMPurify from "isomorphic-dompurify"

/** 与 :::render 生产路径相同：Mustache → DOMPurify。 */
export function mustacheSanitize(template: string, context: Record<string, unknown>): string {
  return DOMPurify.sanitize(Mustache.render(template, context))
}

/**
 * 模板详情预览用的 mock：优先 data.mock（对象或 JSON 字符串），缺省 {}。
 * 非对象 / 解析失败时抛错，由调用方展示提示。
 */
export function parseMockContext(raw: unknown): Record<string, unknown> {
  if (raw == null) return {}
  if (typeof raw === "string") {
    const t = raw.trim()
    if (!t) return {}
    const v: unknown = JSON.parse(t)
    if (v && typeof v === "object" && !Array.isArray(v)) return v as Record<string, unknown>
    throw new Error("data.mock 须为 JSON 对象")
  }
  if (typeof raw === "object" && !Array.isArray(raw)) {
    return raw as Record<string, unknown>
  }
  throw new Error("data.mock 须为 JSON 对象")
}
