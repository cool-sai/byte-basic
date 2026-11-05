import { unified } from "unified"
import remarkParse from "remark-parse"
import remarkGfm from "remark-gfm"
import remarkDirective from "remark-directive"
import React from "react"
import { renderToStaticMarkup } from "react-dom/server"
import ReactMarkdown from "react-markdown"
import { remarkRenderDirective } from "../src/shared/board-view/remark-render-directive.ts"

const md = `:::render{template="steps"}
{"items":[{"n":1,"title":"拉代码"}]}
:::
`
function Probe(props: any) {
  const flat = (c: any): string => {
    if (c == null || typeof c === "boolean") return ""
    if (typeof c === "string" || typeof c === "number") return String(c)
    if (Array.isArray(c)) return c.map(flat).join("")
    if (typeof c === "object" && c.props) return flat(c.props.children)
    return ""
  }
  const s = flat(props.children)
  console.log("flat", s)
  console.log("parse", JSON.parse(s))
  return null
}
renderToStaticMarkup(React.createElement(ReactMarkdown as any, {
  remarkPlugins: [remarkGfm, remarkDirective, remarkRenderDirective],
  components: { "board-template-render": Probe },
}, md))
