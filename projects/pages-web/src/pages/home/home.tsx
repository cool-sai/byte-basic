import { ArrowUpRight, LayoutGrid, Newspaper, NotebookPen, Sparkles, TrendingUp, Workflow, type LucideIcon } from "lucide-react"
import { api, type Json, type Node } from "@/shared/lib/api"
import { fmtTime } from "@/shared/lib/format"
import { useAsync } from "@/shared/lib/use-async"
import { mount } from "@/shared/mount"
import { EmptyState, ErrorState } from "@/shared/states"
import { Skeleton } from "@/shared/ui/skeleton"

// 数据驱动：每个 module（= 一个任务模块）一张大卡片。
// 一次 /modules + 每个模块一次 /{module}?size=1（拿最新一条、总数）。
const SITE = "Agent trace"

/** 模块视觉：图标 + 色相（oklch hue）。没登记的模块用默认。 */
const LOOK: Record<string, { icon: LucideIcon; hue: number }> = {
  news: { icon: Newspaper, hue: 255 },
  finance: { icon: TrendingUp, hue: 150 },
  pipeline: { icon: Workflow, hue: 290 },
  life: { icon: NotebookPen, hue: 35 },
  demo: { icon: Sparkles, hue: 200 },
}
const DEFAULT_LOOK = { icon: LayoutGrid, hue: 290 }
/** 模块没设 data.page 时默认 /{slug}/；首页优先读 data.page。 */
const PAGE_OF: Record<string, string> = {}

const str = (d: Json | undefined, k: string) =>
  d && typeof d === "object" && !Array.isArray(d) && typeof d[k] === "string" ? (d[k] as string) : ""
const hrefOf = (m: Node) => {
  const p = str(m.data, "page")
  try { if (p) return new URL(p, location.href).pathname } catch { /* 忽略非法地址 */ }
  return PAGE_OF[m.slug] ?? `/${m.slug}/`
}

interface Card { m: Node; latest?: Node; total: number }

function ModuleCard({ m, latest, total }: Card) {
  const look = LOOK[m.slug] ?? DEFAULT_LOOK
  const Icon = look.icon
  const c = (l: number, a = 1) => `oklch(${l} 0.16 ${look.hue} / ${a})`
  const desc = str(m.data, "description") || str(m.data, "desc")
  return (
    <a
      href={hrefOf(m)}
      style={{ "--g1": c(0.7, 0.55), "--g2": c(0.7, 0.08), "--glow": c(0.7, 0.25), "--tint": c(0.6) } as React.CSSProperties}
      className="group relative block rounded-2xl p-px transition-all duration-300 [background:linear-gradient(135deg,var(--g1),var(--g2)_45%,var(--g2)_60%,var(--g1))] hover:-translate-y-1 hover:shadow-[0_12px_40px_-12px_var(--glow)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      <div className="relative h-full overflow-hidden rounded-[15px] bg-card p-5 sm:p-6">
        <div aria-hidden className="pointer-events-none absolute -right-16 -top-16 size-48 rounded-full opacity-60 blur-3xl transition-opacity duration-300 group-hover:opacity-100" style={{ background: "var(--glow)" }} />
        <div className="relative flex items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-xl border bg-background/60 shadow-sm" style={{ color: "var(--tint)" }}>
            <Icon className="size-6" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-xl font-semibold tracking-tight">{m.title || m.slug}</h2>
              <span className="rounded-full bg-muted px-2 py-0.5 text-xs tabular-nums text-muted-foreground">{total} 条</span>
            </div>
            {desc && <p className="mt-1 text-sm text-muted-foreground">{desc}</p>}
          </div>
          <ArrowUpRight className="size-5 shrink-0 text-muted-foreground transition-transform duration-300 group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-foreground" />
        </div>
        <div className="relative mt-5 rounded-xl border bg-muted/40 px-4 py-3">
          {latest ? (
            <>
              <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <span className="flex items-center gap-1.5"><span className="size-1.5 rounded-full" style={{ background: "var(--tint)" }} />最新</span>
                <span className="tabular-nums">{fmtTime(latest.updated_at)}</span>
              </div>
              <div className="mt-1 truncate font-medium">{latest.title || latest.slug}</div>
            </>
          ) : <div className="text-sm text-muted-foreground">还没有内容</div>}
        </div>
      </div>
    </a>
  )
}

function HomePage() {
  const st = useAsync(async (signal) => {
    const mods = await api.modules({ size: 100, signal })
    return Promise.all(mods.items.map(async (m): Promise<Card> => {
      try {
        const { details } = await api.module(m.slug, { size: 1, signal })
        return { m, latest: details.items[0], total: details.total }
      } catch { return { m, total: 0 } }
    }))
  }, [])

  return (
    <div className="min-h-screen bg-background">
      <main className="mx-auto max-w-[880px] px-4 pb-16 pt-12 sm:px-6 sm:pt-20">
        <h1 className="mb-8 bg-gradient-to-br from-foreground to-foreground/50 bg-clip-text pb-2 text-5xl font-bold leading-[1.15] tracking-tighter text-transparent sm:mb-12 sm:text-7xl">
          {SITE}
        </h1>
        {st.status === "loading" ? (
          <div className="grid gap-4 sm:grid-cols-2">{[0, 1].map((i) => <Skeleton key={i} className="h-44 rounded-2xl" />)}</div>
        ) : st.status === "error" ? <ErrorState error={st.error} /> : st.data.length === 0 ? <EmptyState title="还没有模块" /> : (
          <div className="grid gap-4 sm:grid-cols-2 sm:gap-5">{st.data.map((c) => <ModuleCard key={c.m.slug} {...c} />)}</div>
        )}
      </main>
    </div>
  )
}

mount(<HomePage />)
