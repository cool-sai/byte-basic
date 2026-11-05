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
  const n = node as { type?: string; value?: string; children?: unknown[] }
  if (n.type === "text" && typeof n.value === "string") return n.value
  if (Array.isArray(n.children)) return n.children.map(extractText).join("")
  return ""
}

/**
 * :::render{template="name"} …JSON… :::
 * → custom hast element board-template-render for ReactMarkdown components.
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
      data.hProperties = { template, context }
      d.children = []
    })
  }
}
