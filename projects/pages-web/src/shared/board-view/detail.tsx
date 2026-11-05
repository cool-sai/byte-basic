import { PageShell } from "@/shared/layout"
import { api, queryParam } from "@/shared/lib/api"
import { fmtTime } from "@/shared/lib/format"
import { useAsync } from "@/shared/lib/use-async"
import { EmptyState, ErrorState, LoadingState } from "@/shared/states"
import { DetailBody } from "./body"
import { moduleSlugFromPath, type GenericDetailData } from "./types"

/** 通用详情：?slug=；正文优先 Markdown，兼容 sections。 */
export function GenericDetailPage() {
  const moduleSlug = moduleSlugFromPath()
  const slug = queryParam("slug") ?? ""
  const st = useAsync((signal) => {
    if (!moduleSlug) return Promise.reject(new Error("无法从 URL 解析 module"))
    if (!slug) return Promise.reject(new Error("缺少参数 ?slug="))
    return api.detail<GenericDetailData>(moduleSlug, slug, { signal })
  }, [moduleSlug, slug])

  const title = st.status === "ok" ? st.data.detail.title || st.data.detail.slug : "详情"
  if (st.status === "ok") document.title = `${title} · ${moduleSlug}`

  const crumbs = [
    { label: "首页", href: "/" },
    { label: moduleSlug || "模块", href: "./" },
    { label: title },
  ]

  if (st.status !== "ok") {
    return (
      <PageShell crumbs={crumbs} title={title}>
        {st.status === "loading" ? (
          <LoadingState rows={5} />
        ) : slug ? (
          <ErrorState error={st.error} />
        ) : (
          <EmptyState title="缺少 ?slug=" />
        )}
      </PageShell>
    )
  }

  const { detail } = st.data
  const d = detail.data ?? {}
  const date = typeof d.date === "string" ? d.date : ""

  return (
    <PageShell
      crumbs={crumbs}
      title={title}
      description={
        <>
          更新于 {fmtTime(detail.updated_at)}
          {date && <> · {date}</>}
        </>
      }
    >
      {d.summary && typeof d.summary === "string" && (
        <p className="mb-6 text-base text-muted-foreground">{d.summary}</p>
      )}
      <DetailBody data={d} moduleSlug={moduleSlug} detailSlug={detail.slug} />
    </PageShell>
  )
}
