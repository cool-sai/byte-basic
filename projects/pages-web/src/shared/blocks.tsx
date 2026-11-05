import { ExternalLink } from "lucide-react"
import type { ReactNode } from "react"
import { api, type Json, type Node } from "@/shared/lib/api"
import { fmtNum, hostOf, humanKey } from "@/shared/lib/format"
import { useAsync } from "@/shared/lib/use-async"
import { Badge } from "@/shared/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/shared/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table"
import { ErrorState, LoadingState } from "@/shared/states"

/* ---------- 形态判断 ---------- */
type Obj = { [k: string]: Json }
export const isObj = (v: Json | undefined): v is Obj => !!v && typeof v === "object" && !Array.isArray(v)
const isScalar = (v: Json | undefined) => v === null || ["string", "number", "boolean"].includes(typeof v)
const isRows = (v: Json | undefined): v is Obj[] => Array.isArray(v) && v.length > 0 && v.every(isObj)
const isLinkItems = (v: Obj[]) => v.every((x) => typeof x.title === "string") && v.some((x) => typeof x.url === "string")

/** 元数据字段不展示（kind/schema/order 等是给程序看的）。 */
const META_KEYS = new Set(["kind", "schema", "schema_version", "order", "id"])

/* ---------- 基础展示件 ---------- */
export function Kpis({ items }: { items: { label: string; value: ReactNode; hint?: ReactNode; tone?: Tone }[] }) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
      {items.map((k) => (
        <div key={k.label} className="rounded-lg border bg-card px-4 py-3">
          <div className="text-xs text-muted-foreground">{k.label}</div>
          <div className={`mt-1 text-2xl font-semibold tabular-nums tracking-tight ${toneText[k.tone ?? "default"]}`}>{k.value}</div>
          {k.hint && <div className="mt-0.5 text-xs text-muted-foreground">{k.hint}</div>}
        </div>
      ))}
    </div>
  )
}

export type Tone = "default" | "success" | "warning" | "danger" | "info"
const toneText: Record<Tone, string> = { default: "", success: "text-success", warning: "text-warning", danger: "text-destructive", info: "text-info" }
const toneBadge: Record<Tone, string> = {
  default: "",
  success: "border-success/30 bg-success/10 text-success",
  warning: "border-warning/30 bg-warning/10 text-warning",
  danger: "border-destructive/30 bg-destructive/10 text-destructive",
  info: "border-info/30 bg-info/10 text-info",
}
/** 中英文状态词 → 色调 */
export function toneOf(v: string): Tone | null {
  if (/亮灯|失败|异常|错误|error|fail/i.test(v)) return "danger"
  if (/警告|注意|接近|传闻|warn|skip/i.test(v)) return "warning"
  if (/完成|正常|成功|通过|done|ok|success|pass/i.test(v)) return "success"
  return null
}
export function StatusBadge({ children, tone }: { children: ReactNode; tone?: Tone | null }) {
  return <Badge variant="outline" className={`font-normal ${toneBadge[tone ?? "default"]}`}>{children}</Badge>
}

export function ValueCell({ v }: { v: Json | undefined }) {
  if (v === null || v === undefined) return <span className="text-muted-foreground">—</span>
  if (typeof v === "number") return <span className="tabular-nums">{fmtNum(v)}</span>
  if (typeof v === "boolean") return <span>{v ? "是" : "否"}</span>
  if (typeof v === "string") {
    if (/^https?:\/\//.test(v)) return <a className="text-info hover:underline" href={v} target="_blank" rel="noreferrer">{hostOf(v) || v}</a>
    const t = v.length <= 12 ? toneOf(v) : null
    return t ? <StatusBadge tone={t}>{v}</StatusBadge> : <span>{v}</span>
  }
  return <code className="text-xs text-muted-foreground">{JSON.stringify(v)}</code>
}

export function DataTable({ rows }: { rows: Obj[] }) {
  const cols = [...new Set(rows.flatMap(Object.keys))].filter((c) => !META_KEYS.has(c))
  const numeric = cols.map((c) => rows.every((r) => r[c] == null || typeof r[c] === "number"))
  return (
    <div className="overflow-hidden rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/50 hover:bg-muted/50">
            {cols.map((c, i) => <TableHead key={c} className={numeric[i] ? "text-right" : ""}>{humanKey(c)}</TableHead>)}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r, ri) => (
            <TableRow key={ri}>
              {cols.map((c, i) => <TableCell key={c} className={numeric[i] ? "text-right" : ""}><ValueCell v={r[c]} /></TableCell>)}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function KeyValue({ data }: { data: Obj }) {
  return (
    <dl className="divide-y rounded-lg border">
      {Object.entries(data).filter(([k]) => !META_KEYS.has(k)).map(([k, v]) => (
        <div key={k} className="grid grid-cols-[minmax(90px,30%)_1fr] gap-3 px-4 py-2.5 text-sm">
          <dt className="text-muted-foreground">{humanKey(k)}</dt>
          <dd className="min-w-0 break-words">{isScalar(v) ? <ValueCell v={v} /> : <GenericData data={v} />}</dd>
        </div>
      ))}
    </dl>
  )
}

/** 链接条目列表：{title, url?, summary?, source?} */
export function LinkList({ items, dense }: { items: Obj[]; dense?: boolean }) {
  return (
    <ul className="divide-y">
      {items.map((it, i) => {
        const url = typeof it.url === "string" ? it.url : undefined
        return (
          <li key={i} className={dense ? "py-2.5 first:pt-0 last:pb-0" : "py-4 first:pt-0 last:pb-0"}>
            {url ? (
              <a href={url} target="_blank" rel="noreferrer" className="group inline-flex items-start gap-1 font-medium leading-snug hover:underline">
                {String(it.title)}<ExternalLink className="mt-1 size-3 shrink-0 opacity-0 transition group-hover:opacity-60" />
              </a>
            ) : (
              <span className="font-medium leading-snug">{String(it.title)}</span>
            )}
            {!dense && typeof it.summary === "string" && <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{it.summary}</p>}
            {(it.source || it.rumor) && (
              <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                {typeof it.source === "string" && <span>{it.source}</span>}
                {it.rumor === true && <StatusBadge tone="warning">待核实</StatusBadge>}
              </div>
            )}
          </li>
        )
      })}
    </ul>
  )
}

/* ---------- 通用自适应渲染 ---------- */
/** 按 JSON 形态自适应：文本→段落；对象数组→链接列表或表格；标量数组→列表；对象→指标 + 键值 + 子区块。 */
export function GenericData({ data }: { data: Json | undefined }): ReactNode {
  if (data === undefined || data === null) return <span className="text-muted-foreground">—</span>
  if (typeof data === "string")
    return <div className="space-y-2 text-sm leading-relaxed">{data.split(/\n+/).map((p, i) => <p key={i}>{p}</p>)}</div>
  if (isScalar(data)) return <ValueCell v={data} />
  if (Array.isArray(data)) {
    if (data.length === 0) return <span className="text-sm text-muted-foreground">（空）</span>
    if (isRows(data)) return isLinkItems(data) ? <LinkList items={data} /> : <DataTable rows={data} />
    if (data.every(isScalar)) return <ul className="list-disc space-y-1 pl-5 text-sm">{data.map((v, i) => <li key={i}><ValueCell v={v} /></li>)}</ul>
    return <pre className="overflow-auto rounded-md bg-muted p-3 text-xs">{JSON.stringify(data, null, 2)}</pre>
  }
  const entries = Object.entries(data).filter(([k]) => !META_KEYS.has(k))
  const nums = entries.filter(([, v]) => typeof v === "number") as [string, number][]
  const scalars = entries.filter(([, v]) => isScalar(v) && typeof v !== "number")
  const nested = entries.filter(([, v]) => !isScalar(v))
  return (
    <div className="space-y-5">
      {nums.length > 0 && <Kpis items={nums.map(([k, v]) => ({ label: humanKey(k), value: fmtNum(v) }))} />}
      {scalars.length > 0 && <KeyValue data={Object.fromEntries(scalars)} />}
      {nested.map(([k, v]) => (
        <div key={k}>
          <h4 className="mb-2 text-xs font-medium text-muted-foreground">{humanKey(k)}</h4>
          <GenericData data={v} />
        </div>
      ))}
    </div>
  )
}

/* ---------- 区块 + 按 schema 覆盖 ---------- */
/** 某种区块的专用渲染器。返回 null 表示“我不处理”，回落到通用渲染。 */
export type BlockRenderer = (data: Json, block: Node) => ReactNode | null
/** key 可以是 data.kind（如 "highlights"）、block.slug 或 "schema:kind"（如 "news.block.v1:quick"）。 */
export type BlockRenderers = Record<string, BlockRenderer>

export function renderBlockData(block: Node, data: Json, renderers: BlockRenderers = {}): ReactNode {
  const kind = isObj(data) && typeof data.kind === "string" ? data.kind : ""
  const schema = isObj(data) && typeof data.schema === "string" ? data.schema : ""
  for (const key of [schema && kind ? `${schema}:${kind}` : "", kind, block.slug]) {
    const r = key && renderers[key]
    if (r) {
      const out = r(data, block)
      if (out !== null) return out
    }
  }
  return <GenericData data={data} />
}

/** 区块卡片：有 data 直接渲染；没有（未用 include=data）则懒加载 /blocks/{slug}。 */
/** at：区块所在的 模块 + 详情 slug，懒加载时用。 */
export interface BlockAt { module: string; detail: string }

export function BlockCard({ block, at, renderers, title, extra }: {
  block: Node; at: BlockAt; renderers?: BlockRenderers; title?: ReactNode; extra?: ReactNode
}) {
  return (
    <Card className="gap-4">
      <CardHeader className="flex flex-row items-center gap-2">
        <CardTitle className="text-base">{title ?? (block.title || block.slug)}</CardTitle>
        {extra && <div className="ml-auto">{extra}</div>}
      </CardHeader>
      <CardContent>
        {block.data !== undefined ? renderBlockData(block, block.data, renderers) : <LazyBlockBody block={block} at={at} renderers={renderers} />}
      </CardContent>
    </Card>
  )
}

function LazyBlockBody({ block, at, renderers }: { block: Node; at: BlockAt; renderers?: BlockRenderers }) {
  const st = useAsync((s) => api.block(at.module, at.detail, block.slug, s), [at.module, at.detail, block.slug])
  if (st.status === "loading") return <LoadingState rows={2} />
  if (st.status === "error") return <ErrorState error={st.error} />
  return <>{renderBlockData(st.data, st.data.data ?? null, renderers)}</>
}
