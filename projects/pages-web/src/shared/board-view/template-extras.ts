import type { Node } from "@/shared/lib/api"
import { hostOf } from "@/shared/lib/format"
import type { GenericDetailData } from "./types"

/** 展示顺序。新闻带 data.order 时以 order 为准，理财/流水线没有 order，按这个排。 */
const KINDS = [
  "highlights",
  "section",
  "quick",
  "health",
  "conclusion",
  "numbers",
  "news_context",
  "holdings_channels",
  "predictions",
  "disclaimer",
  "summary",
  "steps",
] as const

const KIND_RANK = new Map<string, number>(KINDS.map((k, i) => [k, i]))
const nf = new Intl.NumberFormat("zh-CN")

function isObj(v: unknown): v is Record<string, unknown> {
  return !!v && typeof v === "object" && !Array.isArray(v)
}

function mapItem(raw: unknown, index: number, total: number): unknown {
  if (!isObj(raw)) return raw
  const change = typeof raw.change_pct === "number" ? raw.change_pct : null
  const status = typeof raw.status === "string" ? raw.status : ""
  const purchase = typeof raw.purchase_status === "string" ? raw.purchase_status : ""
  const st = status || purchase
  const stopped = /暂停|关闭|stopped/.test(st)
  const limited = /限/.test(st) && !stopped
  const url = typeof raw.url === "string" ? raw.url : ""
  const source = typeof raw.source === "string" ? raw.source : ""
  const view: Record<string, unknown> = {
    ...raw,
    index: index + 1,
    not_last: index < total - 1,
    has_url: url.length > 0,
    host: url ? hostOf(url) : "",
    rumor: raw.rumor === true,
    show_source: source.length > 0 && source !== "pack",
    up: change !== null && change > 0,
    down: change !== null && change < 0,
    done: status === "done",
    failed: status === "failed",
    skipped: status === "skipped",
    waiting: status === "waiting_agent" || status === "blocked",
    node_bad: /fail|error/.test(status),
    node_warn: /skip|warn/.test(status),
    agent: raw.kind === "pe",
    script: raw.kind === "script" || raw.kind === "gate",
    stopped,
    limited,
    state_zh: purchase || (stopped ? "已停" : status === "active" ? "进行中" : status),
    has_daily_limit: typeof raw.daily_limit === "number",
  }
  if (typeof raw.value === "number") view.value_fmt = nf.format(raw.value)
  // 金额不进模板上下文，避免被写进 HTML。
  delete view.amount_cny
  return view
}

function mapList(v: unknown): unknown {
  if (!Array.isArray(v)) return v
  return v.map((item, i) => mapItem(item, i, v.length))
}

function kindOf(b: Node): string {
  const data = isObj(b.data) ? b.data : {}
  return typeof data.kind === "string" && data.kind ? data.kind : b.slug
}

function sortBlocks(blocks: Node[]): Node[] {
  return [...blocks].sort((a, b) => {
    const da = isObj(a.data) ? a.data : {}
    const db = isObj(b.data) ? b.data : {}
    const oa = typeof da.order === "number" ? da.order : null
    const ob = typeof db.order === "number" ? db.order : null
    if (oa !== null && ob !== null && oa !== ob) return oa - ob
    const ra = KIND_RANK.get(kindOf(a)) ?? 100
    const rb = KIND_RANK.get(kindOf(b)) ?? 100
    if (ra !== rb) return ra - rb
    return a.id - b.id
  })
}

/** 把区块整理成模板能直接 {{#is_highlights}} 的视图。 */
export function blockViews(blocks: Node[]): Record<string, unknown>[] {
  const views = sortBlocks(blocks).map((b) => {
    const data = isObj(b.data) ? b.data : {}
    const kind = kindOf(b)
    const view: Record<string, unknown> = {
      ...data,
      slug: b.slug,
      title: b.title || b.slug,
      kind,
      is_ok: data.status === "ok",
      is_failed_run: data.status === "failed",
      is_known: KIND_RANK.has(kind),
    }
    for (const key of ["items", "nodes", "lit_triggers", "dingtou", "fund_status", "new", "fetch_failures"]) {
      if (key in view) {
        view[key] = mapList(view[key])
        view[`has_${key}`] = Array.isArray(view[key]) && (view[key] as unknown[]).length > 0
      }
    }
    for (const name of KINDS) view[`is_${name}`] = kind === name
    if (kind === "health") {
      const nodes = Array.isArray(view.nodes) ? view.nodes : []
      const fails = Array.isArray(view.fetch_failures) ? view.fetch_failures : []
      const doneN = nodes.filter((n) => isObj(n) && n.done === true).length
      view.nodes_done = doneN
      view.nodes_total = nodes.length
      view.is_all_ok = nodes.length > 0 && doneN === nodes.length && fails.length === 0
      view.has_fails = fails.length > 0
    }
    delete view.amount_cny
    return view
  })
  let seenSection = false
  for (const view of views) {
    if (view.is_section === true) {
      view.is_first_section = !seenSection
      seenSection = true
    }
  }
  return views
}

export function templateExtras(data: GenericDetailData | undefined, blocks: Node[]): Record<string, unknown> {
  const view = blockViews(blocks)
  const highlights = view.filter((b) => b.is_highlights)
  const sections = view.filter((b) => b.is_section)
  const quick = view.filter((b) => b.is_quick)
  const scalars: Record<string, unknown> = {}
  if (data) {
    for (const [k, v] of Object.entries(data)) {
      if (k === "md" || k === "content" || k === "sections") continue
      if (typeof v === "string" || typeof v === "number" || typeof v === "boolean") scalars[k] = v
    }
  }
  const extras: Record<string, unknown> = { blocks: view, scalars }
  if (highlights.length || sections.length || quick.length) {
    const count = (list: Record<string, unknown>[]) =>
      list.reduce((n, b) => n + (Array.isArray(b.items) ? b.items.length : 0), 0)
    const itemCountRaw = data && (data as { item_count?: unknown }).item_count
    const itemCount = typeof itemCountRaw === "number" ? itemCountRaw : "—"
    extras.has_kpis = true
    extras.kpis = [
      { label: "入选条目", value: itemCount },
      { label: "今日重点", value: count(highlights) },
      { label: "分区", value: sections.length },
      { label: "速览", value: count(quick) },
    ]
  }
  return extras
}

/** 围栏 JSON 和区块视图合成模板上下文。区块字段由壳补上，围栏里的同名标量仍保留。 */
export function decorateContext(fence: Record<string, unknown>, extras: Record<string, unknown>): Record<string, unknown> {
  const scalars = isObj(extras.scalars) ? extras.scalars : {}
  // 围栏里的字段优先；详情上有、围栏没写的标量（开始时间等）补进来给模板用。
  const ctx: Record<string, unknown> = { ...scalars, ...fence }
  if (Array.isArray(extras.blocks)) ctx.blocks = extras.blocks
  if (extras.has_kpis) {
    ctx.has_kpis = true
    ctx.kpis = extras.kpis
  }
  if (Array.isArray(ctx.highlights)) ctx.highlights = mapList(ctx.highlights)
  if (Array.isArray(ctx.sections)) {
    ctx.sections = ctx.sections.map((sec) => (isObj(sec) ? { ...sec, items: mapList(sec.items) } : sec))
  }
  return ctx
}
