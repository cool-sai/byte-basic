import Mustache from "mustache"
import DOMPurify from "isomorphic-dompurify"
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { api } from "@/shared/lib/api"
import { decorateContext } from "./template-extras"

/** 详情页把区块视图塞进来，:::render 的模板可以 {{#blocks}}。 */
export const TemplateExtras = createContext<Record<string, unknown>>({})

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

function childrenToText(children: ReactNode): string {
  if (children == null || typeof children === "boolean") return ""
  if (typeof children === "string" || typeof children === "number") return String(children)
  if (Array.isArray(children)) return children.map(childrenToText).join("")
  if (typeof children === "object" && children !== null && "props" in children) {
    const props = (children as { props?: { children?: ReactNode } }).props
    return childrenToText(props?.children)
  }
  return ""
}

function parseContext(raw: string): Record<string, unknown> {
  const t = raw.trim()
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
  children?: ReactNode
}

/** 查库 HTML 模板 → Mustache 填数据 → DOMPurify → 插入（无 eval，CSP 安全）。 */
export function BoardTemplateRender({ template = "", children }: Props) {
  const context = useMemo(() => childrenToText(children), [children])
  const extras = useContext(TemplateExtras)
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
        const ctx = decorateContext(parseContext(context), extras)
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
  }, [template, context, extras])

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
