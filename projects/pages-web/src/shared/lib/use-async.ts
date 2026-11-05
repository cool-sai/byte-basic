import { useEffect, useState, type DependencyList } from "react"

export type AsyncState<T> =
  | { status: "loading" }
  | { status: "error"; error: Error }
  | { status: "ok"; data: T }

/** 简单的数据加载 hook：依赖变化时重新加载，组件卸载时取消请求。 */
export function useAsync<T>(fn: (signal: AbortSignal) => Promise<T>, deps: DependencyList): AsyncState<T> {
  const [state, setState] = useState<AsyncState<T>>({ status: "loading" })
  useEffect(() => {
    const ctl = new AbortController()
    setState({ status: "loading" })
    fn(ctl.signal).then(
      (data) => !ctl.signal.aborted && setState({ status: "ok", data }),
      (error) => !ctl.signal.aborted && setState({ status: "error", error: error instanceof Error ? error : new Error(String(error)) }),
    )
    return () => ctl.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
  return state
}
