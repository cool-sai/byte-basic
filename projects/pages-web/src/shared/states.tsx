import { AlertCircle, Inbox } from "lucide-react"
import type { ReactNode } from "react"
import { Button } from "@/shared/ui/button"
import { Card, CardContent } from "@/shared/ui/card"
import { Skeleton } from "@/shared/ui/skeleton"
import type { AsyncState } from "@/shared/lib/use-async"

export function LoadingState({ rows = 3 }: { rows?: number }) {
  return (
    <Card>
      <CardContent className="space-y-4">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className="space-y-2">
            <Skeleton className="h-4 w-3/5" />
            <Skeleton className="h-3 w-4/5" />
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

export function EmptyState({ title = "暂无内容", description }: { title?: string; description?: ReactNode }) {
  return (
    <Card>
      <CardContent className="flex flex-col items-center py-10 text-center">
        <Inbox className="mb-3 size-8 text-muted-foreground/60" />
        <p className="font-medium">{title}</p>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </CardContent>
    </Card>
  )
}

export function ErrorState({ error, onRetry }: { error: Error; onRetry?: () => void }) {
  return (
    <Card className="border-destructive/40">
      <CardContent className="flex flex-col items-center py-10 text-center">
        <AlertCircle className="mb-3 size-8 text-destructive" />
        <p className="font-medium">加载失败</p>
        <p className="mt-1 break-all text-sm text-muted-foreground">{error.message}</p>
        <Button variant="outline" size="sm" className="mt-4" onClick={onRetry ?? (() => location.reload())}>
          重试
        </Button>
      </CardContent>
    </Card>
  )
}

/** 按 useAsync 的状态渲染：加载中 / 出错 / 正常。 */
export function AsyncView<T>({ state, children, loading }: { state: AsyncState<T>; children: (data: T) => ReactNode; loading?: ReactNode }) {
  if (state.status === "loading") return <>{loading ?? <LoadingState />}</>
  if (state.status === "error") return <ErrorState error={state.error} />
  return <>{children(state.data)}</>
}
