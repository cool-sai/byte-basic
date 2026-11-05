import { unified } from "unified"
import remarkParse from "remark-parse"
import remarkGfm from "remark-gfm"
import remarkDirective from "remark-directive"
import { visit } from "unist-util-visit"

const md = `:::render{template="steps"}
{"items":[{"n":1,"title":"a"}]}
:::
`
const tree = unified().use(remarkParse).use(remarkGfm).use(remarkDirective).parse(md)
console.log(JSON.stringify(tree, null, 2))

const md2 = `:::render{template="steps"}
\`\`\`json
{"items":[{"n":1,"title":"a"}]}
\`\`\`
:::
`
const tree2 = unified().use(remarkParse).use(remarkGfm).use(remarkDirective).parse(md2)
visit(tree2, (n: any) => {
  if (n.type === "containerDirective") console.log("code way", JSON.stringify(n, null, 2))
})
