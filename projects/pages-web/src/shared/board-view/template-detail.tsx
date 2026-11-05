import { useMemo } from "react"
import { SectionTitle } from "@/shared/layout"
import { Card } from "@/shared/ui/card"
import { mustacheSanitize, parseMockContext } from "./mustache-html"
import type { TemplateDetailData } from "./types"

function SourceBlock({ title, empty, children }: { title: string; empty?: string; children?: string }) {
  const text = children?.trim() ? children : ""
  return (
    <section>
      <SectionTitle>{title}</SectionTitle>
      <Card className="gap-0 overflow-hidden py-0">
        {text ? (
          <pre className="m-0 overflow-x-auto p-4 text-[0.85em] leading-relaxed whitespace-pre-wrap break-words">
            {text}
          </pre>
        ) : (
          <p className="px-4 py-3 text-sm text-muted-foreground">{empty || "（空）"}</p>
        )}
      </Card>
    </section>
  )
}

/** templates 模块详情：Mock 数据 / 模板 / 渲染效果。 */
export function TemplateDetailBody({ data }: { data?: TemplateDetailData }) {
  const html = typeof data?.html === "string" ? data.html : ""
  const mockRaw = data?.mock

  const mockView = useMemo(() => {
    try {
      const ctx = parseMockContext(mockRaw)
      return { ok: true as const, ctx, text: JSON.stringify(ctx, null, 2) }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "mock 无效"
      return { ok: false as const, msg, text: typeof mockRaw === "string" ? mockRaw : JSON.stringify(mockRaw, null, 2) }
    }
  }, [mockRaw])

  const renderView = useMemo(() => {
    if (!html.trim()) return { ok: false as const, msg: "无 data.html" }
    if (!mockView.ok) return { ok: false as const, msg: mockView.msg }
    try {
      return { ok: true as const, html: mustacheSanitize(html, mockView.ctx) }
    } catch (e) {
      return { ok: false as const, msg: e instanceof Error ? e.message : "渲染失败" }
    }
  }, [html, mockView])

  return (
    <div className="space-y-8">
      <SourceBlock title="Mock 数据" empty="无 data.mock，预览用 {}">
        {mockView.text}
      </SourceBlock>

      <SourceBlock title="模板" empty="Agent 请在 data.html 写入 Mustache HTML">
        {html}
      </SourceBlock>

      <section>
        <SectionTitle>渲染效果</SectionTitle>
        <Card className="gap-0 overflow-hidden py-0">
          {renderView.ok ? (
            <div
              className="board-template-render p-4 text-sm leading-relaxed"
              dangerouslySetInnerHTML={{ __html: renderView.html }}
            />
          ) : (
            <p className="px-4 py-3 text-sm text-muted-foreground">未渲染：{renderView.msg}</p>
          )}
        </Card>
      </section>
    </div>
  )
}
