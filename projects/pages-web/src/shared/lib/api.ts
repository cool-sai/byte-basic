// board 只读内容 API 客户端。
// 生产默认同域 `/api/data`（pages nginx 剥掉 /api → board `/data`）。
// 覆盖：
//   VITE_API_BASE   完整前缀，如 `/api/data` 或 `http://localhost:18090/data`
//   VITE_API_ORIGIN 非空时拼 `${origin}/data`（本地直连 board，或过渡期 `https://lssl.work/board`）
function resolveApiBase(): string {
  const base = import.meta.env.VITE_API_BASE
  if (base != null && String(base).length > 0) {
    return String(base).replace(/\/+$/, "")
  }
  const origin = import.meta.env.VITE_API_ORIGIN
  if (origin != null && String(origin) !== "") {
    return `${String(origin).replace(/\/+$/, "")}/data`
  }
  return "/api/data"
}

export const API_BASE: string = resolveApiBase()
/** @deprecated 使用 API_BASE；保留别名以免外部误引用。 */
export const API_ORIGIN: string = API_BASE

export type Json = null | boolean | number | string | Json[] | { [k: string]: Json }

export interface Node<D = Json> {
  id: number
  parent_id: number
  type: "module" | "list" | "detail" | "block"
  slug: string
  title: string
  data?: D
  schema_version: number
  created_at: string
  updated_at: string
}

export interface NodePage<D = Json> {
  items: Node<D>[]
  page: number
  page_size: number
  total: number
}

export class ApiError extends Error {
  status: number
  constructor(path: string, status: number) {
    super(`${path} → HTTP ${status}`)
    this.status = status
  }
}

/** 本地假数据：VITE_MOCK=1 时由页面注册，按 API 路径（不含查询串）返回；线上构建不会用到。 */
export const mock: { routes?: Record<string, unknown> } = {}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  if (mock.routes) {
    const hit = mock.routes[path.split("?")[0]]
    if (hit === undefined) throw new ApiError(path, 404)
    return structuredClone(hit) as T
  }
  const res = await fetch(API_BASE + path, { signal })
  if (!res.ok) throw new ApiError(path, res.status)
  return res.json() as Promise<T>
}

const q = (page?: number, size?: number) => {
  const p = new URLSearchParams()
  if (page) p.set("page", String(page))
  if (size) p.set("size", String(size))
  const s = p.toString()
  return s ? `?${s}` : ""
}

const enc = encodeURIComponent

// 两级内容树：module → detail → block，全部按名字（slug）访问。
export const api = {
  /** 全部模块（首页导航）。 */
  modules: (o: { page?: number; size?: number; signal?: AbortSignal } = {}) =>
    get<NodePage>(`/modules${q(o.page, o.size)}`, o.signal),

  /** 模块 + 其下详情（分页，新到旧，不含区块）。 */
  module: <D = Json>(module: string, o: { page?: number; size?: number; signal?: AbortSignal } = {}) =>
    get<{ module: Node; details: NodePage<D> }>(`/${enc(module)}${q(o.page, o.size)}`, o.signal),

  /** 详情 + 区块。includeData=true（默认）时区块带 data，一次拿全；false 时只有清单，用 block() 懒加载。 */
  detail: <D = Json, B = Json>(module: string, slug: string, o: { includeData?: boolean; signal?: AbortSignal } = {}) =>
    get<{ detail: Node<D>; blocks: Node<B>[] }>(`/${enc(module)}/${enc(slug)}${o.includeData === false ? "" : "?include=data"}`, o.signal),

  /** 单个区块（含 data），用于懒加载大区块。 */
  block: <B = Json>(module: string, slug: string, block: string, signal?: AbortSignal) =>
    get<Node<B>>(`/${enc(module)}/${enc(slug)}/blocks/${enc(block)}`, signal),
}

export function queryParam(name: string): string | null {
  return new URLSearchParams(location.search).get(name)
}
