import type { Root } from "mdast"
import type { Plugin } from "unified"
import { visit } from "unist-util-visit"

type DirectiveNode = {
  type: string
  name?: string
  attributes?: Record<string, string | null | undefined>
  children?: unknown[]
  data?: { hName?: string; hProperties?: Record<string, string> }
}

function extractText(node: unknown): string {
  if (!node || typeof node !== "object") return ""
  const n = node as { type?: string; value?: string; name?: string; children?: unknown[] }
  // fenced code inside :::render keeps JSON intact
  if (n.type === "code" && typeof n.value === "string") return n.value
  if (n.type === "text" && typeof n.value === "string") return n.value
  // remark-directive treats ":1" in {"n":1} as textDirective — restore ":name"
  if (n.type === "textDirective" && typeof n.name === "string") return `:${n.name}`
  if (Array.isArray(n.children)) return n.children.map(extractText).join("")
  return ""
}

/**
 * :::render{template="name"} …JSON… :::
 * → board-template-render；JSON 留在 children 文本，不进 HTML 属性。
 */
export const remarkRenderDirective: Plugin<[], Root> = () => {
  return (tree) => {
    visit(tree, (node) => {
      const d = node as DirectiveNode
      if (
        (d.type !== "containerDirective" && d.type !== "leafDirective" && d.type !== "textDirective") ||
        d.name !== "render"
      ) {
        return
      }
      const template = String(d.attributes?.template ?? "").trim()
      const context = extractText(d).trim()
      const data = d.data ?? (d.data = {})
      data.hName = "board-template-render"
      data.hProperties = { template }
      // 直接文本子节点，避免 HTML attr 转义破坏 JSON
      d.children = context ? [{ type: "text", value: context }] : []
    })
  }
}
