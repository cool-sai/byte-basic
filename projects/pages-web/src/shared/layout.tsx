import { Fragment, type ReactNode } from "react"
import {
  Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator,
} from "@/shared/ui/breadcrumb"

export interface Crumb { label: string; href?: string }

/** 统一页面骨架：面包屑 + 标题 + 说明 + 右侧附加信息，内容最大宽度 880px。 */
export function PageShell(props: {
  crumbs: Crumb[]
  title: ReactNode
  description?: ReactNode
  aside?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="min-h-screen bg-background">
      <main className="mx-auto max-w-[880px] px-4 py-6 sm:px-6 sm:py-10">
        <Breadcrumb className="mb-4">
          <BreadcrumbList>
            {props.crumbs.map((c, i) => (
              <Fragment key={i}>
                {i > 0 && <BreadcrumbSeparator />}
                <BreadcrumbItem>
                  {c.href ? <BreadcrumbLink href={c.href}>{c.label}</BreadcrumbLink> : <BreadcrumbPage>{c.label}</BreadcrumbPage>}
                </BreadcrumbItem>
              </Fragment>
            ))}
          </BreadcrumbList>
        </Breadcrumb>
        <header className="mb-8 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{props.title}</h1>
            {props.description && <p className="mt-1.5 text-sm text-muted-foreground">{props.description}</p>}
          </div>
          {props.aside && <div className="shrink-0">{props.aside}</div>}
        </header>
        {props.children}
      </main>
    </div>
  )
}

/** 分区标题：小号灰字 + 可选计数。 */
export function SectionTitle({ children, count, extra }: { children: ReactNode; count?: number; extra?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center gap-2">
      <h2 className="text-sm font-medium text-muted-foreground">{children}</h2>
      {count !== undefined && <span className="text-xs tabular-nums text-muted-foreground/70">{count}</span>}
      {extra && <div className="ml-auto">{extra}</div>}
    </div>
  )
}
