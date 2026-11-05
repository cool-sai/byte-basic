# pages-web

`pages.lssl.work` 的独立静态站（B2）：nginx 托管 home/shell，并把 `/api/data/` 反代到 board 的 `/data/`。

## 行为

| 浏览器 | 处理 |
|---|---|
| `/` | `dist/home/index.html`（资源在 `/home/assets/…`） |
| `/{module}/`、`/{module}/detail.html`、`/{module}/assets/…` | 回落到 `dist/shell/` 同名文件 |
| `/api/data/…` | `proxy_pass` → `http://board:8090/data/…` |
| `/news/daily/`、`/finance/digest/`、`/life/daily/` | 301 → `/news/` 等 |

CSP：`connect-src 'self'`（同域 API，不再依赖 `lssl.work/board`）。

## 部署（赛梁）

1. 合并并部署 **board**（本 PR 的 `/data` 路由）。
2. 构建前端并同步到宿主机：
   ```bash
   # 在 cool-sai/pages
   npm run build -- home
   npm run build -- shell
   rsync -a --delete dist/home/ /srv/pages/home/
   rsync -a --delete dist/shell/ /srv/pages/shell/
   ```
3. 在平台 compose 里启用 `projects/pages-web/compose.yml`（与 board 同网络，能解析 `board`）。
4. **TLB**：`pages.lssl.work` 上游从 `board:8090` 改为 `pages-web:80`。
5. 主站 `lssl.work/board` 不动（MCP / put_page / 过渡期 `/api/v2`）。

未切 TLB 前，旧 put_page 壳仍可工作；切完后不再需要按模块 put_page。
