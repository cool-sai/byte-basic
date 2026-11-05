# pages-web

`pages.lssl.work` 的**单一部署单元**（B2）：本目录同时含 React 源码、nginx 与多阶段 Docker。镜像构建时 `npm run build` 出 home/shell，nginx 托管产物，并把 `/api/data/` 反代到 board `/data/`。

源码曾在独立仓 `cool-sai/pages`；已迁入本目录，该仓仅作 archive 说明。

## 行为

| 浏览器 | 处理 |
|---|---|
| `/` | `dist/home/index.html`（资源在 `/home/assets/…`） |
| `/{module}/`、`/{module}/detail.html`、`/{module}/assets/…` | 回落到 `dist/shell/` 同名文件 |
| `/api/data/…` | `proxy_pass` → `http://board:8090/data/…` |
| `/news/daily/`、`/finance/digest/`、`/life/daily/` | 301 → `/news/` 等 |

CSP：`connect-src 'self'`（同域 API）。

## 目录

```
src/shared/            共用（api / layout / board-view / ui）
src/pages/home/        首页
src/pages/shell/       通用列表 + 详情壳
scripts/build.mjs      构建一个页面目标
scripts/publish.mjs    过渡期 put_page（B2 切 TLB 后一般不用）
nginx.conf             静态回落 + /api/data 反代
Dockerfile             node 构建 → nginx 镜像
compose.yml            与 board 同网络
```

## 本地开发

```bash
cd projects/pages-web
npm ci
npm run build -- shell          # → dist/shell/，API=/api/data
npm run build -- home
PAGE_TARGET=shell npx vite      # 热更新；/api/data 代理到 localhost:18090
```

## 部署（赛梁）

一条命令打镜像即可（源码在仓内，无需另仓 rsync dist）：

1. 合并并部署 **board**（本 PR 的 `/data` 路由）。
2. 平台启用 `projects/pages-web/compose.yml`（与 board 同网络，能解析 `board`）→ 构建本 Dockerfile。
3. **TLB**：`pages.lssl.work` 上游从 `board:8090` 改为 `pages-web:80`。
4. 主站 `lssl.work/board` 不动（MCP / put_page / 过渡期 `/api/v2`）。

改壳或首页：改本目录源码 → 重新构建 pages-web 镜像。加模块内容：只需 `put_node`，不必 `put_page`。
