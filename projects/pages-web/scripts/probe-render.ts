import { unified } from "unified"
import remarkParse from "remark-parse"
import remarkGfm from "remark-gfm"
import remarkDirective from "remark-directive"
import { visit } from "unist-util-visit"
import { remarkRenderDirective } from "../src/shared/board-view/remark-render-directive.ts"
import React from "react"
import { renderToStaticMarkup } from "react-dom/server"
import ReactMarkdown from "react-markdown"

const md = `:::render{template="steps"}
{"items":[{"n":1,"title":"拉代码","detail":"git pull / 合 PR","tag":"Git"}]}
:::
`

const tree2 = unified().use(remarkParse).use(remarkGfm).use(remarkDirective).parse(md)
unified().use(remarkParse).use(remarkGfm).use(remarkDirective).use(remarkRenderDirective).runSync(tree2)
visit(tree2, (n: any) => {
  if (n.data?.hName === "board-template-render" || n.name === "render") {
    console.log("NODE", JSON.stringify(n, null, 2))
  }
})

function Probe(props: any) {
  console.log("PROPS keys", Object.keys(props))
  console.log("template", props.template)
  console.log("context", props.context)
  const flat = (c: any): string => {
    if (c == null || typeof c === "boolean") return ""
    if (typeof c === "string" || typeof c === "number") return String(c)
    if (Array.isArray(c)) return c.map(flat).join("")
    if (typeof c === "object" && c.props) return flat(c.props.children)
    return ""
  }
  console.log("flat children", JSON.stringify(flat(props.children)))
  return null
}

renderToStaticMarkup(
  React.createElement(
    ReactMarkdown as any,
    {
      remarkPlugins: [remarkGfm, remarkDirective, remarkRenderDirective],
      components: { "board-template-render": Probe },
    },
    md,
  ),
)
