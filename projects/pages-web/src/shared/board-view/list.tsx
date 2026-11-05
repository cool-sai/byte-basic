import { ChevronLeft, ChevronRight, ChevronRightIcon } from "lucide-react"
import { useEffect, useState } from "react"
import { PageShell, SectionTitle } from "@/shared/layout"
import { api, queryParam } from "@/shared/lib/api"
import { useAsync } from "@/shared/lib/use-async"
import { AsyncView, EmptyState } from "@/shared/states"
import { Badge } from "@/shared/ui/badge"
import { Button } from "@/shared/ui/button"
import { Card } from "@/shared/ui/card"
import { moduleSlugFromPath, type GenericDetailData } from "./types"

const SIZE = 20

function str(d: GenericDetailData | undefined, k: keyof GenericDetailData): string {
  const v = d?.[k]
  return typeof v === "string" ? v : ""
}

/** 通用列表：module 从 URL 第一段解析，可 put_page 到任意单段 path。 */
export function GenericListPage() {
  const moduleSlug = moduleSlugFromPath()
  const [page, setPage] = useState(() => Math.max(1, Number(queryParam("page")) || 1))
  useEffect(() => {
    const onPop = () => setPage(Math.max(1, Number(queryParam("page")) || 1))
    addEventListener("popstate", onPop)
    return () => removeEventListener("popstate", onPop)
  }, [])
  const go = (p: number) => {
    history.pushState(null, "", p > 1 ? `?page=${p}` : location.pathname)
    setPage(p)
    scrollTo(0, 0)
  }

  const st = useAsync(async (signal) => {
    if (!moduleSlug) throw new Error("无法从 URL 解析 module（需要 /{module}/）")
    return api.module<GenericDetailData>(moduleSlug, { page, size: SIZE, signal })
  }, [moduleSlug, page])

  const modTitle = (st.status === "ok" && (st.data.module.title || st.data.module.slug)) || moduleSlug || "模块"
  const desc =
    st.status === "ok"
      ? (() => {
          const d = st.data.module.data
          if (d && typeof d === "object" && !Array.isArray(d)) {
            const s = d.description ?? d.desc
            if (typeof s === "string") return s
          }
          return "列表 · 点击查看详情"
        })()
      : "列表 · 点击查看详情"

  if (st.status === "ok") document.title = modTitle

  return (
    <PageShell crumbs={[{ label: "首页", href: "/" }, { label: modTitle }]} title={modTitle} description={desc}>
      <AsyncView state={st}>
        {({ details }) => {
          const pages = Math.max(1, Math.ceil(details.total / SIZE))
          if (!details.items.length)
            return <EmptyState title="还没有内容" description="用 put_node 写入 type=detail 后会出现在这里" />
          return (
            <>
              <SectionTitle count={details.total}>全部</SectionTitle>
              <Card className="gap-0 overflow-hidden py-0">
                <ul className="divide-y">
                  {details.items.map((d, i) => {
                    const data = d.data ?? {}
                    const date = str(data, "date") || (/^\d{4}-\d{2}-\d{2}/.test(d.slug) ? d.slug : "")
                    const summary = str(data, "summary")
                    return (
                      <li key={d.id}>
                        <a
                          href={`detail.html?slug=${encodeURIComponent(d.slug)}`}
                          className="group flex items-center gap-4 px-5 py-4 transition-colors hover:bg-muted/50"
                        >
                          {date ? (
                            <div className="w-12 shrink-0 text-center">
                              <div className="text-xl font-semibold tabular-nums leading-none">{date.slice(8, 10) || "·"}</div>
                              <div className="mt-1 text-[11px] tabular-nums text-muted-foreground">{date.slice(0, 7)}</div>
                            </div>
                          ) : (
                            <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted text-xs font-medium text-muted-foreground">
                              {(d.title || d.slug).slice(0, 1)}
                            </div>
                          )}
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                              <span className="truncate font-medium">{d.title || d.slug}</span>
                              {i === 0 && page === 1 && <Badge className="shrink-0">最新</Badge>}
                            </div>
                            {summary && <p className="mt-1 truncate text-sm text-muted-foreground">{summary}</p>}
                          </div>
                          <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground/60 transition group-hover:translate-x-0.5" />
                        </a>
                      </li>
                    )
                  })}
                </ul>
              </Card>
              {pages > 1 && (
                <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
                  <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => go(page - 1)}>
                    <ChevronLeft />
                    上一页
                  </Button>
                  <span className="tabular-nums">
                    第 {page} / {pages} 页
                  </span>
                  <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => go(page + 1)}>
                    下一页
                    <ChevronRight />
                  </Button>
                </div>
              )}
            </>
          )
        }}
      </AsyncView>
    </PageShell>
  )
}
