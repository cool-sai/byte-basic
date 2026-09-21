# board

`lssl.work` 上的刷题看板：浏览器读数，Grok / 其它 agent 走远程 MCP 写数。

- 页面：https://lssl.work/board/
- 种类列表：https://lssl.work/board/kinds/（点进去分页看条目）
- Token 管理：https://lssl.work/board/tokens/
- MCP 工具目录：https://lssl.work/board/tools/
- 只读：`GET /board/api/kv/stats`（按种类 COUNT，不鉴权）
- MCP：`https://lssl.work/board/mcp`（Bearer：管理员 token 或签发出来的 named token）

MySQL 三张表：`board_kind`（种类）、`board_item`（列表：id / content / type / date / extra）、`board_kind_html`（种类绑定的 html 片段，用来渲染 content）。

工具：`create_kind`、`add_item`、`update_item`、`delete_item`、`list_items`、`create_kind_html`、`update_kind_html`、`delete_kind_html`、`get_kind_html`。

## Token

环境变量 `BOARD_TOKEN` 是管理员密钥，用来打开 `/board/tokens/`，不发给 Grok / agent。

给客户端用的 token 在管理页生成：填名字（例如 `grok bot`）→ 生成 → **明文只显示一次**。列表里只留名称、前缀、创建时间、最近使用，用来对上是谁在用。

也可以命令行签发：

```bash
curl -s https://lssl.work/board/api/tokens \
  -H "Authorization: Bearer $BOARD_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"grok bot"}'
```

## 环境

```
LISTEN=0.0.0.0:8090
BOARD_TOKEN=<管理员密钥，openssl rand -hex 16>
MYSQL=root:minikitex@tcp(mysql:3306)/minikitex?parseTime=true&charset=utf8mb4
TOKENS_FILE=/data/tokens.json
```

仓库根目录 `.env`：

```
BOARD_TOKEN=管理员密钥
```

## 不用 agent 验收

本机 compose 的 TLB 在 `:80`。公网 `https://lssl.work` 若还没指到这台机器，用下面 Host 头测：

```bash
# 探活（本机）
curl -s -H 'Host: lssl.work' http://127.0.0.1/board/api/health

# 探活（域名已指过来时）
curl -s https://lssl.work/board/api/health
```

其余命令把 URL 换成你实际打得通的那条即可。

```bash
# 探活
curl -s https://lssl.work/board/api/health

# 无 token / 错 token → 401
curl -sI -X POST https://lssl.work/board/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream'
curl -sI -X POST https://lssl.work/board/mcp \
  -H "Authorization: Bearer wrong" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream'

curl -s https://lssl.work/board/mcp \
  -H "Authorization: Bearer $BOARD_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_kind","arguments":{"name":"算法"}}}'

curl -s https://lssl.work/board/mcp \
  -H "Authorization: Bearer $BOARD_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"add_item","arguments":{"kind":"算法","content":"两数之和","date":"2026-09-21","type":"数组"}}}'

curl -s https://lssl.work/board/api/kv/stats
# 浏览器打开 https://lssl.work/board/
```

## Grok Bot / Grok CLI

```toml
[mcp_servers.board]
url = "https://lssl.work/board/mcp"
headers = { "Authorization" = "Bearer ${BOARD_NAMED_TOKEN}" }
```

把 `${BOARD_NAMED_TOKEN}` 换成管理页生成的那一串（不要用管理员 `BOARD_TOKEN`）。

```bash
grok mcp add --transport http board https://lssl.work/board/mcp \
  --header "Authorization: Bearer ${BOARD_NAMED_TOKEN}"
grok mcp doctor board
```

工具名：`create_kind`、`add_item`、`update_item`、`delete_item`、`list_items`、`create_kind_html`、`update_kind_html`、`delete_kind_html`、`get_kind_html`（Grok 目录里是 `board__add_item` 这种）。

Cursor 填同一个 URL + `Authorization` header。
