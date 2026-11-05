import { ExternalLink } from "lucide-react"
import type { Components } from "react-markdown"
import ReactMarkdown from "react-markdown"
import remarkDirective from "remark-directive"
import remarkGfm from "remark-gfm"
import { SectionTitle } from "@/shared/layout"
import { EmptyState } from "@/shared/states"
import { Card } from "@/shared/ui/card"
import { remarkRenderDirective } from "./remark-render-directive"
import { BoardTemplateRender } from "./template-render"
import { markdownOf, type GenericDetailData, type GenericSection } from "./types"

const mdClass = [
  "prose-board text-sm leading-relaxed text-foreground",
  "[&_h1]:mb-3 [&_h1]:mt-6 [&_h1]:text-2xl [&_h1]:font-semibold [&_h1]:tracking-tight [&_h1:first-child]:mt-0",
  "[&_h2]:mb-2 [&_h2]:mt-5 [&_h2]:text-xl [&_h2]:font-semibold",
  "[&_h3]:mb-2 [&_h3]:mt-4 [&_h3]:text-lg [&_h3]:font-medium",
  "[&_p]:my-2.5",
  "[&_ul]:my-2.5 [&_ul]:list-disc [&_ul]:pl-5",
  "[&_ol]:my-2.5 [&_ol]:list-decimal [&_ol]:pl-5",
  "[&_li]:my-1",
  "[&_strong]:font-semibold",
  "[&_a]:text-primary [&_a]:underline-offset-2 hover:[&_a]:underline",
  "[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-muted-foreground/40 [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
  "[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-[0.85em]",
  "[&_pre]:my-3 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:bg-muted [&_pre]:p-3 [&_pre]:text-[0.85em]",
  "[&_pre_code]:bg-transparent [&_pre_code]:p-0",
  "[&_table]:my-3 [&_table]:w-full [&_table]:border-collapse [&_table]:text-sm",
  "[&_th]:border [&_th]:border-border [&_th]:bg-muted/50 [&_th]:px-3 [&_th]:py-1.5 [&_th]:text-left [&_th]:font-medium",
  "[&_td]:border [&_td]:border-border [&_td]:px-3 [&_td]:py-1.5",
  "[&_hr]:my-6 [&_hr]:border-border",
  "[&_del]:text-muted-foreground [&_del]:line-through",
  "[&_input]:mr-2",
].join(" ")

const mdComponents = {
  "board-template-render": BoardTemplateRender,
} as Components & {
  "board-template-render": typeof BoardTemplateRender
}

function SectionsBody({ sections }: { sections: GenericSection[] }) {
  const nonempty = sections.filter((s) => (s.items?.length ?? 0) > 0)
  if (!nonempty.length) return <EmptyState title="这份详情还没有内容" />
  return (
    <div className="space-y-8">
      {nonempty.map((sec, i) => (
        <section key={sec.key || sec.title || i}>
          <SectionTitle count={sec.items!.length}>{sec.title || sec.key || `分区 ${i + 1}`}</SectionTitle>
          <Card className="gap-0 overflow-hidden py-0">
            <ul className="divide-y">
              {sec.items!.map((item, j) => (
                <li key={j} className="px-5 py-3.5">
                  {item.link ? (
                    <a
                      href={item.link}
                      target="_blank"
                      rel="noreferrer"
                      className="group inline-flex items-start gap-2 text-sm hover:underline"
                    >
                      <span>{item.text || item.link}</span>
                      <ExternalLink className="mt-0.5 size-3.5 shrink-0 text-muted-foreground opacity-60 transition group-hover:opacity-100" />
                    </a>
                  ) : (
                    <p className="text-sm">{item.text || ""}</p>
                  )}
                </li>
              ))}
            </ul>
          </Card>
        </section>
      ))}
    </div>
  )
}

/** 详情正文：md/content → GFM + :::render；否则 sections[]；再否则 summary / 空。 */
export function DetailBody({ data }: { data?: GenericDetailData }) {
  const md = markdownOf(data)
  if (md) {
    return (
      <div className={mdClass}>
        <ReactMarkdown
          remarkPlugins={[remarkGfm, remarkDirective, remarkRenderDirective]}
          components={mdComponents}
        >
          {md}
        </ReactMarkdown>
      </div>
    )
  }
  if (data?.sections?.length) return <SectionsBody sections={data.sections} />
  if (data?.summary) return <p className="text-base text-muted-foreground">{data.summary}</p>
  return <EmptyState title="无正文" description="Agent 可在 data.md 或 data.content 写入 GFM Markdown" />
}
