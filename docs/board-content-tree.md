# board 内容树（v2）与 Agent 页面托管

在 `projects/board` 里新增，和旧的 kind/item 看板并存，旧表、旧接口、旧 MCP 工具都不动。

## 1. 数据模型

一张通用节点表，三种类型，两级（module → detail），层级固定：

```
module（模块，如 finance）
└── detail（详情，如 digest-30-evening）
    └── block（区块，如 conclusion），data 为任意 JSON
```

旧版的 list 层（module → list → detail）已删除。board 启动时 `flattenLists` 会把历史数据里 list 下的 detail 改挂到 module 下，再软删除空的 list（幂等；同名冲突的 detail 跳过并打日志，该 list 保留待人工处理）。模块名 `modules` 是保留字。

`board_node`

| 列 | 说明 |
|---|---|
| id | 自增主键 |
| parent_id | 父节点 id，module 为 0 |
| type | module / detail / block |
| slug | 同一父节点下唯一（`uk_parent_slug`），小写字母数字 `. _ -` |
| title | 展示标题 |
| data | JSON，任意结构；页面按结构自适应 |
| schema_version | data 的结构版本，默认 1；改结构就 +1，页面发现版本不认识就提示「数据格式变了」 |
| deleted_at | 软删除时间；删除节点会连同整棵子树一起软删除 |
| created_at / updated_at | |

索引：`(parent_id, type, created_at)`，唯一键 `(parent_id, slug)`。

页面两张表：

- `board_page(path PK, current_version)`：当前上线版本指针
- `board_page_version(path, version, files LONGTEXT JSON, created_at)`，唯一键 `(path, version)`：每次上传一个新版本，不覆盖旧版本

三张表都在 board 启动时 `CREATE TABLE IF NOT EXISTS`，和旧表同库（minikitex）。

## 2. 只读接口（公开，不鉴权，CORS `*`，仅 GET）

浏览器路径带 `/board` 前缀（TLB 剥掉后进程内是 `/api/v2/...`）：

| 接口 | 返回 |
|---|---|
| `GET /board/api/v2/modules?page=&size=` | 模块分页（首页导航用） |
| `GET /board/api/v2/{module}?page=&size=` | `{module, details}`，模块下的详情，新到旧，size 默认 20、最大 100，不含区块 |
| `GET /board/api/v2/{module}/{slug}` | `{detail, blocks}`；blocks 只有清单，不含 data |
| `GET /board/api/v2/{module}/{slug}?include=data` | 同上，但每个区块都带 data，一次拿全（每 100 个区块一条查询，最多 1000 个）。`include` 只认 `data` |
| `GET /board/api/v2/{module}/{slug}/blocks/{block}` | 单个区块，含 data |

全部按名字（slug），不用数字 id。不存在、已删除、路径不对，一律 JSON 404（以前错路径会因为 `OPTIONS /api/v2/` 兜底变成 405）。

## 3. MCP 写工具（`https://lssl.work/board/mcp`，Bearer token）

| 工具 | 作用 |
|---|---|
| `put_node(type, slug, parent_id? 或 module/detail, title, data, schema_version)` | 按 父节点+slug 幂等创建/覆盖，已删除的会复活，返回节点（含 id）。父节点可用 `parent_id`，或按名字：detail 填 `module`，block 填 `module`+`detail` |
| `update_node(id, title?, data?, schema_version?)` | 只改传入的字段，data 整体替换 |
| `delete_node(id)` | 软删除整棵子树，返回删除数量 |
| `get_node(id)` / `list_nodes(parent_id, type, page, page_size)` | 读 |
| `put_page(path, files{name: content})` | 保存为新版本并立即上线；必须含 index.html |
| `list_page_versions(path)` | 历史版本，新到旧，标出 current |
| `rollback_page(path, version)` | current 指针切回旧版本 |

典型写法：`put_node(type=module, slug=finance)` → `put_node(type=detail, module=finance, slug=digest-30-evening, data={…})` → `put_node(type=block, module=finance, detail=digest-30-evening, slug=conclusion, data={…})`。

## 4. 页面托管

- 访问：`https://pages.lssl.work/<path>/`（配置了 `PAGES_HOST`）；没配时走 `https://lssl.work/board/pages/<path>/`（仅开发用，见下）。
- 页面路径 1~4 段（`finance/drawdown`），文件名可带最多 3 层子目录，禁止 `..`、绝对路径、反斜杠；单页最多 64 个文件、合计 4MB。
- `GET /<path>` 自动补 `/`，相对引用 `./app.js` 才能解析对。

环境变量：

| 变量 | 默认 | 作用 |
|---|---|---|
| `PAGES_HOST` | 空 | 页面专用域名，如 `pages.lssl.work` |
| `PAGES_API_ORIGIN` | `https://lssl.work` | 写进 CSP `connect-src`，页面 JS 只能连这里 |
| `PAGES_EXTRA_SRC` | 空 | 额外允许的脚本/样式/字体来源，如 `https://cdn.jsdelivr.net` |

## 5. 安全设计

1. **页面与主站不同源。** 设了 `PAGES_HOST` 后，页面只在独立子域名提供；主域名访问 `/board/pages/...` 会 302 过去。Agent 写的 JS 即使有问题，也拿不到 lssl.work 上的任何东西（token 管理页的 localStorage 等）。
2. **页面域名只暴露页面。** 请求 Host 是 `PAGES_HOST` 时，只交给页面处理器，且只允许 GET/HEAD；`/mcp`、`/api/tokens`、`/api/v2` 在这个域名下都是 404/405。
3. **CSP**：`default-src 'none'`；脚本/样式只能来自自身（允许 inline）及 `PAGES_EXTRA_SRC`；`connect-src` 只有读接口源；`frame-ancestors 'none'`、`form-action 'none'`、`base-uri 'none'`、`object-src 'none'`；另加 `X-Frame-Options: DENY`、`X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`。没配独立域名时额外加 `sandbox allow-scripts`，让页面落在不透明源。
4. **写只认 Bearer 头。** board 全程不读 cookie；MCP 只校验 `Authorization: Bearer`，管理员 token 或签发的 named token。读接口不带凭据（CORS 不开 credentials）。
5. **页面 JS 只能读。** 读接口是公开 GET；写接口在 MCP 上，需要 token，而 token 不会出现在页面里（页面也不该放 token）。
6. **可回滚。** 页面每次上传一个新版本，`rollback_page` 改指针即可；节点删除是软删除，同 slug 再 put 会复活。

## 6. 局限

- MySQL 实现没有自动化测试（测试用的是同语义的内存实现）；上线后按下面的「验证」手测一次。
- 页面版本不会自动清理；以后需要可以加「保留最近 N 个」。
- 读接口公开：不要往内容树里写不想公开的数据。
