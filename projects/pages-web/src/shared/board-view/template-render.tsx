import Mustache from "mustache"
import DOMPurify from "isomorphic-dompurify"
import { useEffect, useState } from "react"
import { api } from "@/shared/lib/api"

/** 本页生命周期内缓存模板 HTML（按 slug）。 */
const templateCache = new Map<string, Promise<string>>()

function loadTemplateHtml(name: string): Promise<string> {
  const hit = templateCache.get(name)
  if (hit) return hit
  const p = api
    .detail<{ html?: unknown }>("templates", name)
    .then((res) => {
      const html = res.detail?.data?.html
      if (typeof html !== "string" || !html.trim()) {
        throw new Error("无 data.html")
      }
      return html
    })
  templateCache.set(name, p)
  p.catch(() => {
    templateCache.delete(name)
  })
  return p
}

function base64UrlToUtf8(b64url: string): string {
  const b64 = b64url.replace(/-/g, "+").replace(/_/g, "/")
  const pad = b64.length % 4 === 0 ? "" : "=".repeat(4 - (b64.length % 4))
  const bin = atob(b64 + pad)
  const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

function decodeContextRaw(raw: string): string {
  const t = raw.trim()
  if (!t) return ""
  if (t.startsWith("b64:")) return base64UrlToUtf8(t.slice(4))
  // legacy: HTML-attribute round-trip may turn " into &quot;
  return t
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&")
}

function parseContext(raw: string): Record<string, unknown> {
  const t = decodeContextRaw(raw)
  if (!t) return {}
  try {
    const v: unknown = JSON.parse(t)
    if (v && typeof v === "object" && !Array.isArray(v)) {
      return v as Record<string, unknown>
    }
    throw new Error("上下文须为 JSON 对象")
  } catch (e) {
    throw e instanceof SyntaxError ? new Error("上下文 JSON 无效") : e
  }
}

type Props = {
  template?: string
  context?: string
  children?: unknown
}

/** 查库 HTML 模板 → Mustache 填数据 → DOMPurify → 插入（无 eval，CSP 安全）。 */
export function BoardTemplateRender({ template = "", context = "" }: Props) {
  const [html, setHtml] = useState<string | null>(null)
  const [status, setStatus] = useState<"loading" | "ok" | "error">("loading")
  const [message, setMessage] = useState("")

  useEffect(() => {
    let cancelled = false
    setStatus("loading")
    setHtml(null)
    setMessage("")

    void (async () => {
      const name = template.trim()
      if (!name) {
        if (!cancelled) {
          setStatus("error")
          setMessage("缺少 template")
        }
        return
      }
      try {
        const tpl = await loadTemplateHtml(name)
        const ctx = parseContext(context)
        // Mustache 走 token walk，不用 new Function / eval
        const rendered = Mustache.render(tpl, ctx)
        const clean = DOMPurify.sanitize(rendered)
        if (!cancelled) {
          setHtml(clean)
          setStatus("ok")
        }
      } catch (e) {
        if (!cancelled) {
          setStatus("error")
          const msg = e instanceof Error ? e.message : "渲染失败"
          setMessage(msg.includes("HTTP 404") ? "模板不存在" : msg)
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [template, context])

  if (status === "loading") {
    return <p className="my-2 text-sm text-muted-foreground">加载模板…</p>
  }
  if (status === "error") {
    return (
      <p className="my-2 text-sm text-muted-foreground">
        模板「{template || "?"}」未渲染{message ? `：${message}` : ""}
      </p>
    )
  }
  return (
    <div
      className="board-template-render my-3 text-sm leading-relaxed"
      dangerouslySetInnerHTML={{ __html: html ?? "" }}
    />
  )
}
