# Agent 内容写入约定（一期）

页面壳只读、不写。内容进 board 内容树（`put_node`）。壳与首页由 `projects/pages-web` 镜像构建托管；B2 切 TLB 后加模块一般不必 `put_page`。

## 三层结构

```
module（模块）
  └── detail（列表里的一行 = 一篇详情）
        └── block（可选；本期详情正文不依赖 block）
```

- **module**：一个领域入口，例如 `life`、`demo`、`rating`
- **detail**：列表项，点进去是详情页
- **block**：子块；一期详情优先把 Markdown 放在 `detail.data` 上，可不建 block

每条节点都有：`slug`（同父下唯一）、`title`、`data`（JSON）、`parent` 关系。

## 写模块

```
put_node
  type: module
  slug: demo
  title: 演示
  data:
    description: 一句话说明
    page: /demo/          # 可选；缺省首页会链到 /{slug}/
```

## 写详情（推荐 Markdown）

```
put_node
  type: detail
  module: demo            # 所属模块 slug
  slug: hello             # 列表/URL 用
  title: 你好，Markdown
  data:
    summary: 列表上显示的一句话   # 可选
    date: "2026-11-06"          # 可选，YYYY-MM-DD
    md: |                        # 或用 content，二选一即可
      # 标题
      - 列表
      | 列 | 值 |
      | --- | --- |
      | GFM | 表格 |
```

正文字段优先级（页面渲染）：

1. `data.md`（非空字符串）
2. `data.content`（非空字符串）
3. 否则若有 `data.sections[]`（旧 life 形），按分区列表渲染
4. 再否则展示 `summary`，都没有则显示「无正文」

Markdown 按 **GFM**（GitHub Flavored Markdown）渲染：表格、任务列表、删除线、自动链接等。

## URL

| 页 | 地址 |
| --- | --- |
| 列表 | `https://pages.lssl.work/<module>/` |
| 详情 | `https://pages.lssl.work/<module>/detail.html?slug=<detail-slug>` |
| 首页 | `https://pages.lssl.work/`（读 modules；`data.page` 或默认 `/{slug}/`） |

通用壳从 URL **第一段**解析 module。仓内**不再按模块堆目录**。

## 页面壳 vs 内容

| 动作 | 工具 | 作用 |
| --- | --- | --- |
| 上传/更新静态页 | `put_page` | `index.html` + js/css，立即上线 |
| 写/覆盖内容节点 | `put_node` | module / detail / block |
| 改已有节点 | `update_node` | 按 id 改 title / data |
| 列/读 | `list_nodes` / `get_node` | 或页面走公开 `/api/data`（board `/data`） |

## 仓内目录（pages）

```
src/pages/home/     首页
src/pages/shell/    通用列表+详情壳（构建目标名 shell）
src/shared/board-view/  真正的列表/详情逻辑
```

构建：`npm run build -- shell` → `dist/shell/`。

## 新模块怎么加（不用改仓）

1. `put_node` 建 `type=module`（可选 `data.page` / `data.description`）
2. `put_node` 写若干 `type=detail`（`module=<slug>`，正文放 `data.md` 或兼容 `sections`）
3. **壳**（二选一）：
   - **board 已部署「_shell 回落」后**：把 `dist/shell/` `put_page` 到 path=`_shell` **一次即可**；之后新模块不用再 put_page
   - **过渡期**（board 未部署回落）：把同一套 `dist/shell/` 再 `put_page` 到该模块 slug（path 单段，如 `news`）

## 已迁通用壳

`demo`、`life`、`news`、`finance`、`pipeline` 均走通用壳（列表 + GFM/sections 详情）。

### 功能损失（相对旧专用壳）

- **news**：不再有晨报专用区块排版（highlight / quick 等 block UI）；详情优先看 `summary`/`md`，旧数据多为 block，页面上会偏「简」
- **finance**：digest 卡片/KPI/仓位等专用块不再渲染，只剩列表摘要级信息
- **pipeline**：Run 流程图 / runs 视图消失，详情只显示 data 里能落到 md/sections/summary 的部分

旧路径重定向：`/news/daily/` → `/news/`，`/finance/digest/` → `/finance/`，`/life/daily/` → `/life/`。

## HTML 模板（:::render）

详情 Markdown 里可用 directive 拉取 **module=`templates`** 下的 HTML 片段，用 **Mustache** 填数据后消毒插入（**会剥掉 `<script>` 等**）。Mustache 解释模板、不走 `eval`/`new Function`，兼容 CSP（无 `unsafe-eval`）。

### 写模板（存库）

```
put_node
  type: module
  slug: templates
  title: HTML 模板
  data:
    description: 详情 :::render 用的 HTML 片段
```

每个模板是一条 detail，slug = 模板名，HTML 放在 `data.html`：

```
put_node
  type: detail
  module: templates
  slug: steps
  title: 步骤列表
  data:
    html: |
      <ol class="board-steps">
      {{#items}}
        <li><strong>{{title}}</strong>{{#detail}} — {{.}}{{/detail}}</li>
      {{/items}}
      {{^items}}
        <li>暂无步骤</li>
      {{/items}}
      </ol>
```

循环 / 条件用 [Mustache](https://mustache.github.io/mustache.5.html)（**不是** Handlebars）：

- 循环：`{{#items}}…{{title}}…{{/items}}`（数组节；不要写 `{{#each items}}`）
- 条件（真值 / 非空）：`{{#detail}}…{{/detail}}`；节内当前值用 `{{.}}`
- 否定 / 空列表：`{{^items}}…{{/items}}`
- 插值：`{{title}}`（默认转义）；原样 HTML 用 `{{{html}}}`（仍会再过 DOMPurify）

壳请求：`GET /api/data/templates/<slug>?include=data`（与其它模块相同）。

### 在详情 Markdown 里引用

```
:::render{template="steps"}
{"items":[{"title":"拉代码","detail":"clone"},{"title":"测试"},{"title":"部署"}]}
:::
```

- `template` = `templates` 模块下 detail 的 slug
- 围栏正文 = 可选 **JSON 对象**，作为 Mustache 上下文；空正文或省略正文则用 `{}`
- 加载中 / 找不到模板 / JSON 无效：详情页显示一行灰色提示，不整页崩溃
- 同一页内同名模板只拉一次（内存缓存）

无 body 亦可：

```
:::render{template="steps"}
:::
```

## 未做（二期及以后）

- 在线 HTML 组件库、按 module 换列表/详情组件
- 详情页用户互动写回（打分、加运动员）
- board `_shell` 回落合并上线后，可删掉各模块 path 上重复的 put_page（只留 `_shell`）
